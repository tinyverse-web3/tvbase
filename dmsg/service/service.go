package service

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"time"

	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	tvCommon "github.com/tinyverse-web3/tvbase/common"
	"github.com/tinyverse-web3/tvbase/common/db"
	"github.com/tinyverse-web3/tvbase/dmsg"
	dmsgKey "github.com/tinyverse-web3/tvbase/dmsg/common/key"
	dmsgLog "github.com/tinyverse-web3/tvbase/dmsg/common/log"
	dmsgUser "github.com/tinyverse-web3/tvbase/dmsg/common/user"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol"
	customProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol/custom"
	dmsgServiceCommon "github.com/tinyverse-web3/tvbase/dmsg/service/common"
	serviceProtocol "github.com/tinyverse-web3/tvbase/dmsg/service/protocol"
	tvutilKey "github.com/tinyverse-web3/tvutil/key"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type ProtocolProxy struct {
	createMailboxProtocol *dmsgServiceCommon.StreamProtocol
	releaseMailboxPrtocol *dmsgServiceCommon.StreamProtocol
	readMailboxMsgPrtocol *dmsgServiceCommon.StreamProtocol
	createChannelProtocol *dmsgServiceCommon.StreamProtocol
	seekMailboxProtocol   *dmsgServiceCommon.PubsubProtocol
	queryPeerProtocol     *dmsgServiceCommon.PubsubProtocol
	sendMsgProtocol       *serviceProtocol.SendMsgProtocol
}

type DmsgService struct {
	dmsg.DmsgService
	ProtocolProxy
	datastore                    db.Datastore
	user                         *dmsgUser.LightUser
	destUserList                 map[string]*dmsgServiceCommon.DestUserInfo
	channelInfoList              map[string]*dmsgServiceCommon.PubChannel
	customProtocolPubsubList     map[string]*dmsgServiceCommon.CustomProtocolPubsub
	protocolReqSubscribes        map[pb.PID]protocol.ReqSubscribe
	protocolResSubscribes        map[pb.PID]protocol.ResSubscribe
	customStreamProtocolInfoList map[string]*dmsgServiceCommon.CustomStreamProtocolInfo
	customPubsubProtocolInfoList map[string]*dmsgServiceCommon.CustomPubsubProtocolInfo
	stopCleanRestResource        chan bool
}

func CreateService(nodeService tvCommon.TvBaseService) (*DmsgService, error) {
	d := &DmsgService{}
	err := d.Init(nodeService)
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (d *DmsgService) Init(nodeService tvCommon.TvBaseService) error {
	err := d.DmsgService.Init(nodeService)
	if err != nil {
		return err
	}

	cfg := d.BaseService.GetConfig()
	d.datastore, err = db.CreateDataStore(cfg.DMsg.DatastorePath, cfg.Mode)
	if err != nil {
		dmsgLog.Logger.Errorf("dmsgService->Init: create datastore error %v", err)
		return err
	}

	// stream protocol
	d.createMailboxProtocol = serviceProtocol.NewCreateMailboxProtocol(d.BaseService.GetCtx(), d.BaseService.GetHost(), d, d)
	d.releaseMailboxPrtocol = serviceProtocol.NewReleaseMailboxProtocol(d.BaseService.GetCtx(), d.BaseService.GetHost(), d, d)
	d.readMailboxMsgPrtocol = serviceProtocol.NewReadMailboxMsgProtocol(d.BaseService.GetCtx(), d.BaseService.GetHost(), d, d)
	d.createChannelProtocol = serviceProtocol.NewCreateChannelProtocol(d.BaseService.GetCtx(), d.BaseService.GetHost(), d, d)

	// pubsub protocol
	d.seekMailboxProtocol = serviceProtocol.NewSeekMailboxProtocol(d.BaseService.GetCtx(), d.BaseService.GetHost(), d, d)
	d.RegPubsubProtocolReqCallback(d.seekMailboxProtocol.Adapter.GetRequestPID(), d.seekMailboxProtocol)

	// sendMsgProtocol, special pubsub protocol
	d.sendMsgProtocol = serviceProtocol.NewSendMsgProtocol(d.BaseService.GetHost(), d, d)
	d.RegPubsubProtocolReqCallback(pb.PID_SEND_MSG_REQ, d.sendMsgProtocol)

	d.destUserList = make(map[string]*dmsgServiceCommon.DestUserInfo)
	d.customProtocolPubsubList = make(map[string]*dmsgServiceCommon.CustomProtocolPubsub)

	d.channelInfoList = make(map[string]*dmsgServiceCommon.PubChannel)

	d.protocolReqSubscribes = make(map[pb.PID]protocol.ReqSubscribe)
	d.protocolResSubscribes = make(map[pb.PID]protocol.ResSubscribe)

	d.customStreamProtocolInfoList = make(map[string]*dmsgServiceCommon.CustomStreamProtocolInfo)
	d.customPubsubProtocolInfoList = make(map[string]*dmsgServiceCommon.CustomPubsubProtocolInfo)

	d.stopCleanRestResource = make(chan bool)
	return nil
}

func (d *DmsgService) InitUser(userPubkeyData []byte, getSig dmsgKey.GetSigCallback) error {
	dmsgLog.Logger.Debug("DmsgService->InitUser begin")
	userPubkey := tvutilKey.TranslateKeyProtoBufToString(userPubkeyData)
	err := d.subscribeUser(userPubkey, getSig)
	if err != nil {
		return err
	}

	dmsgLog.Logger.Debug("DmsgService->InitUser end")
	return nil
}

// ProtocolService
func (d *DmsgService) GetUserPubkeyHex() (string, error) {
	if d.user == nil {
		return "", fmt.Errorf("DmsgService->GetUserPubkeyHex: user is nil")
	}
	return d.user.Key.PubkeyHex, nil
}

func (d *DmsgService) GetUserSig(protoData []byte) ([]byte, error) {
	if d.user == nil {
		dmsgLog.Logger.Errorf("DmsgService->GetUserSig: user is nil")
		return nil, fmt.Errorf("DmsgService->GetUserSig: user is nil")
	}
	return d.user.GetSig(protoData)
}

func (d *DmsgService) getMsgPrefix(userPubkey string) string {
	return dmsg.MsgPrefix + userPubkey
}

func (d *DmsgService) saveUserMsg(protoMsg protoreflect.ProtoMessage) error {
	sendMsgReq, ok := protoMsg.(*pb.SendMsgReq)
	if !ok {
		dmsgLog.Logger.Errorf("dmsgService->saveUserMsg: cannot convert %v to *pb.SendMsgReq", protoMsg)
		return fmt.Errorf("dmsgService->saveUserMsg: cannot convert %v to *pb.SendMsgReq", protoMsg)
	}

	pubkey := sendMsgReq.BasicData.Pubkey
	pubsub := d.getDestUserPubsub(pubkey)
	if pubsub == nil {
		dmsgLog.Logger.Errorf("dmsgService->saveUserMsg: cannot find src user pubsub for %v", pubkey)
		return fmt.Errorf("dmsgService->saveUserMsg: cannot find src user pubsub for %v", pubkey)
	}

	pubsub.MsgRWMutex.RLock()
	defer pubsub.MsgRWMutex.RUnlock()

	key := d.GetFullFromMsgPrefix(sendMsgReq)
	err := d.datastore.Put(d.BaseService.GetCtx(), ds.NewKey(key), sendMsgReq.Content)
	if err != nil {
		return err
	}

	return err
}

func (d *DmsgService) isAvailableMailbox(userPubKey string) bool {
	destUserCount := len(d.destUserList)
	cfg := d.BaseService.GetConfig()
	return destUserCount < cfg.DMsg.MaxMailboxPubsubCount
}

func (d *DmsgService) cleanRestResource(done chan bool) {
	go func() {
		ticker := time.NewTicker(3 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				cfg := d.BaseService.GetConfig()
				for userPubkey, pubsub := range d.destUserList {
					days := daysBetween(pubsub.LastReciveTimestamp, time.Now().Unix())
					// delete mailbox msg in datastore and cancel mailbox subscribe when days is over, default days is 30
					if days >= cfg.DMsg.KeepMailboxMsgDay {
						var query = query.Query{
							Prefix:   d.getMsgPrefix(userPubkey),
							KeysOnly: true,
						}
						results, err := d.datastore.Query(d.BaseService.GetCtx(), query)
						if err != nil {
							dmsgLog.Logger.Errorf("dmsgService->readDestUserPubsub: query error: %v", err)
						}

						for result := range results.Next() {
							d.datastore.Delete(d.BaseService.GetCtx(), ds.NewKey(result.Key))
							dmsgLog.Logger.Debugf("dmsgService->readDestUserPubsub: delete msg by key:%v", string(result.Key))
						}

						d.unSubscribeDestUser(userPubkey)
						return
					}
				}
				for pubChannelPubkey, pubsub := range d.channelInfoList {
					days := daysBetween(pubsub.LastReciveTimestamp, time.Now().Unix())
					// delete mailbox msg in datastore and cancel mailbox subscribe when days is over, default days is 30
					if days >= cfg.DMsg.KeepPubChannelDay {
						d.unsubscribePubChannel(pubChannelPubkey)
						return
					}
				}

				continue
			case <-d.BaseService.GetCtx().Done():
				return
			}
		}
	}()
}

func (d *DmsgService) readProtocolPubsub(pubsub *dmsgServiceCommon.UserPubsub) {
	ctx, cancel := context.WithCancel(d.BaseService.GetCtx())
	pubsub.CancelFunc = cancel
	for {
		m, err := pubsub.Subscription.Next(ctx)
		if err != nil {
			dmsgLog.Logger.Warnf("dmsgService->readDestUserPubsub: subscription.Next happen err, %v", err)
			return
		}
		if d.BaseService.GetHost().ID() == m.ReceivedFrom {
			continue
		}

		dmsgLog.Logger.Debugf("dmsgService->readDestUserPubsub: receive pubsub msg, topic:%s, receivedFrom:%v", *m.Topic, m.ReceivedFrom)

		protocolID, protocolIDLen, err := d.CheckPubsubData(m.Data)
		if err != nil {
			dmsgLog.Logger.Errorf("dmsgService->readDestUserPubsub: CheckPubsubData error %v", err)
			continue
		}
		contentData := m.Data[protocolIDLen:]
		reqSubscribe := d.PubsubProtocolReqSubscribes[protocolID]
		if reqSubscribe != nil {
			err = reqSubscribe.HandleRequestData(contentData)
			if err != nil {
				dmsgLog.Logger.Warnf("dmsgService->readDestUserPubsub: HandleRequestData error %v", err)
			}
			continue
		} else {
			dmsgLog.Logger.Warnf("dmsgService->readDestUserPubsub: no find protocolId(%d) for reqSubscribe", protocolID)
		}
		// no protocol for resSubScribe in service
		resSubScribe := d.PubsubProtocolResSubscribes[protocolID]
		if resSubScribe != nil {
			err = resSubScribe.HandleResponseData(contentData)
			if err != nil {
				dmsgLog.Logger.Warnf("dmsgService->readDestUserPubsub: HandleResponseData error %v", err)
			}
			continue
		} else {
			dmsgLog.Logger.Warnf("dmsgService->readDestUserPubsub: no find protocolId(%d) for resSubscribe", protocolID)
		}
	}
}

// destuser
func (d *DmsgService) publishDestUser(destUserPubkey string) error {
	pubsub := d.getDestUserPubsub(destUserPubkey)
	if pubsub != nil {
		dmsgLog.Logger.Errorf("dmsgService->publishDestUser: user public key(%s) pubsub already exist", destUserPubkey)
		return fmt.Errorf("dmsgService->publishDestUser: user public key(%s) pubsub already exist", destUserPubkey)
	}
	err := d.subscribeDestUser(destUserPubkey)
	if err != nil {
		return err
	}
	pubsub = d.getDestUserPubsub(destUserPubkey)
	go d.readDestUserPubsub(pubsub)
	return nil
}

func (d *DmsgService) unPublishDestUser(destUserPubkey string) error {
	pubsub := d.getDestUserPubsub(destUserPubkey)
	if pubsub == nil {
		return nil
	}
	err := d.unSubscribeDestUser(destUserPubkey)
	if err != nil {
		return err
	}
	return nil
}

func (d *DmsgService) readDestUserPubsub(pubsub *dmsgServiceCommon.DestUserInfo) {
	d.readProtocolPubsub(&pubsub.UserPubsub)
}

func (d *DmsgService) subscribeDestUser(userPubKey string) error {
	var err error
	userTopic, err := d.Pubsub.Join(userPubKey)
	if err != nil {
		return err
	}
	userSub, err := userTopic.Subscribe()
	if err != nil {
		return err
	}
	// go d.BaseService.DiscoverRendezvousPeers()

	d.destUserList[userPubKey] = &dmsgServiceCommon.DestUserInfo{
		UserPubsub: dmsgServiceCommon.UserPubsub{
			Topic:        userTopic,
			Subscription: userSub,
		},
		MsgRWMutex:          sync.RWMutex{},
		LastReciveTimestamp: time.Now().Unix(),
	}
	return nil
}

func (d *DmsgService) unSubscribeDestUser(userPubKey string) error {
	pubsub := d.destUserList[userPubKey]
	if pubsub == nil {
		dmsgLog.Logger.Errorf("dmsgService->unSubscribeDestUser: no find userPubkey %s for pubsub", userPubKey)
		return fmt.Errorf("dmsgService->unSubscribeDestUser: no find userPubkey %s for pubsub", userPubKey)
	}
	pubsub.CancelFunc()
	pubsub.Subscription.Cancel()
	err := pubsub.Topic.Close()
	if err != nil {
		dmsgLog.Logger.Warnf("dmsgService->unSubscribeDestUser: Topic.Close error: %v", err)
	}
	delete(d.destUserList, userPubKey)
	return nil
}

func (d *DmsgService) unSubscribeDestUsers() error {
	for userPubKey := range d.destUserList {
		d.unSubscribeDestUser(userPubKey)
	}
	return nil
}

func (d *DmsgService) getDestUserPubsub(destUserPubsub string) *dmsgServiceCommon.DestUserInfo {
	return d.destUserList[destUserPubsub]
}

// pubChannel
func (d *DmsgService) isAvailablePubChannel(pubKey string) bool {
	pubChannelInfo := len(d.channelInfoList)
	cfg := d.BaseService.GetConfig()
	if pubChannelInfo >= cfg.DMsg.MaxPubChannelPubsubCount {
		dmsgLog.Logger.Warnf("dmsgService->isAvailableMailbox: exceeded the maximum number of mailbox services, current destUserCount:%v", pubChannelInfo)
		return false
	}
	return true
}

func (d *DmsgService) subscribePubChannel(pubkey string) error {
	pubsub := d.channelInfoList[pubkey]
	if pubsub != nil {
		dmsgLog.Logger.Errorf("dmsgService->SubscribePubChannel: public key(%s) pubsub already exist", pubkey)
		return fmt.Errorf("dmsgService->SubscribePubChannel: public key(%s) pubsub already exist", pubkey)
	}

	userTopic, err := d.Pubsub.Join(pubkey)
	if err != nil {
		dmsgLog.Logger.Errorf("dmsgService->SubscribePubChannel: Join error: %v", err)
		return err
	}
	userSub, err := userTopic.Subscribe()
	if err != nil {
		dmsgLog.Logger.Errorf("dmsgService->SubscribePubChannel: Subscribe error: %v", err)
		return err
	}
	// go d.BaseService.DiscoverRendezvousPeers()

	d.channelInfoList[pubkey] = &dmsgServiceCommon.PubChannel{
		UserPubsub: dmsgServiceCommon.UserPubsub{
			Topic:        userTopic,
			Subscription: userSub,
		},
		LastReciveTimestamp: time.Now().Unix(),
	}

	// go d.readPubChannelPubsub(d.pubChannelInfoList[pubkey])
	return nil
}

func (d *DmsgService) unsubscribePubChannel(pubkey string) error {
	pubsub := d.channelInfoList[pubkey]
	if pubsub == nil {
		dmsgLog.Logger.Errorf("dmsgService->unSubscribeDestUser: no find pubkey %s for pubsub", pubkey)
		return fmt.Errorf("dmsgService->unSubscribeDestUser: no find pubkey %s for pubsub", pubkey)
	}
	pubsub.CancelFunc()
	pubsub.Subscription.Cancel()
	err := pubsub.Topic.Close()
	if err != nil {
		dmsgLog.Logger.Warnf("dmsgService->unSubscribeDestUser: Topic.Close error: %v", err)
	}
	delete(d.channelInfoList, pubkey)
	return nil
}

func (d *DmsgService) unsubscribePubChannelList() error {
	for pubkey := range d.channelInfoList {
		d.unsubscribePubChannel(pubkey)
	}
	return nil
}

func (d *DmsgService) readPubChannelPubsub(pubsub *dmsgServiceCommon.PubChannel) {
	d.readProtocolPubsub(&pubsub.UserPubsub)
}

// for sdk
func (d *DmsgService) Start() error {
	d.cleanRestResource(d.stopCleanRestResource)
	return nil
}

func (d *DmsgService) Stop() error {
	d.unsubscribePubChannelList()
	d.unSubscribeDestUsers()
	d.stopCleanRestResource <- true
	return nil
}

func (d *DmsgService) GetDestUserPubsub(publicKey string) *dmsgServiceCommon.DestUserInfo {
	return d.destUserList[publicKey]
}

// StreamProtocolCallback interface
func (d *DmsgService) OnCreateMailboxRequest(requestProtoData protoreflect.ProtoMessage) (any, any, error) {
	dmsgLog.Logger.Debugf("dmsgService->OnCreateMailboxRequest begin:\nrequestProtoData:%v", requestProtoData)
	request, ok := requestProtoData.(*pb.CreateMailboxReq)
	if !ok {
		dmsgLog.Logger.Errorf("dmsgService->OnCreateMailboxRequest: fail to convert to *pb.CreateMailboxReq")
		return nil, nil, fmt.Errorf("dmsgService->OnCreateMailboxRequest: fail to convert to *pb.CreateMailboxReq")
	}
	isAvailable := d.isAvailableMailbox(request.BasicData.Pubkey)
	if !isAvailable {
		dmsgLog.Logger.Errorf("dmsgService->OnCreateMailboxRequest: exceeded the maximum number of mailbox service")
		return nil, nil, errors.New("dmsgService->OnCreateMailboxRequest: exceeded the maximum number of mailbox service")
	}
	pubsub := d.getDestUserPubsub(request.BasicData.Pubkey)
	if pubsub != nil {
		dmsgLog.Logger.Errorf("dmsgService->OnCreateMailboxRequest: user public key pubsub already exist")
		retCode := &pb.RetCode{
			Code:   dmsgServiceCommon.MailboxAlreadyExistCode,
			Result: "dmsgService->OnCreateMailboxRequest: user public key pubsub already exist",
		}
		return nil, retCode, nil
	}

	err := d.publishDestUser(request.BasicData.Pubkey)
	if err != nil {
		return nil, nil, err
	}
	dmsgLog.Logger.Debugf("dmsgService->OnCreateMailboxRequest end")
	return nil, nil, nil
}

func (d *DmsgService) OnCreateMailboxResponse(protoData protoreflect.ProtoMessage) (any, error) {
	return nil, nil
}

func (d *DmsgService) OnReleaseMailboxRequest(requestProtoData protoreflect.ProtoMessage) (any, any, error) {
	dmsgLog.Logger.Debugf("dmsgService->OnReleaseMailboxRequest begin:\nReleaseMailboxRequest:%v", requestProtoData)
	request, ok := requestProtoData.(*pb.ReleaseMailboxReq)
	if !ok {
		dmsgLog.Logger.Errorf("dmsgService->OnReleaseMailboxRequest: cannot convert to *pb.ReleaseMailboxReq")
		return nil, nil, fmt.Errorf("dmsgService->OnReleaseMailboxRequest: cannot convert to *pb.ReleaseMailboxReq")
	}

	err := d.unPublishDestUser(request.BasicData.Pubkey)
	if err != nil {
		return nil, nil, err
	}
	dmsgLog.Logger.Debugf("dmsgService->OnReleaseMailboxRequest end")
	return nil, nil, nil
}
func (d *DmsgService) OnReadMailboxMsgRequest(requestProtoData protoreflect.ProtoMessage) (any, any, error) {
	dmsgLog.Logger.Debugf("dmsgService->OnReadMailboxMsgRequest begin:\nrequestProtoData:%v", requestProtoData)
	request, ok := requestProtoData.(*pb.ReadMailboxReq)
	if !ok {
		dmsgLog.Logger.Errorf("dmsgService->OnReadMailboxMsgRequest: cannot convert to *pb.ReadMailboxReq")
		return nil, nil, fmt.Errorf("dmsgService->OnReadMailboxMsgRequest: cannot convert to *pb.ReadMailboxReq")
	}

	pubkey := request.BasicData.Pubkey
	pubsub := d.getDestUserPubsub(pubkey)
	if pubsub == nil {
		dmsgLog.Logger.Errorf("dmsgService->OnReadMailboxMsgRequest: cannot find src user pubsub for %s", pubkey)
		return nil, nil, fmt.Errorf("dmsgService->OnReadMailboxMsgRequest: cannot find src user pubsub for %s", pubkey)
	}
	pubsub.MsgRWMutex.RLock()
	defer pubsub.MsgRWMutex.RUnlock()

	var query = query.Query{
		Prefix: d.getMsgPrefix(pubkey),
	}
	results, err := d.datastore.Query(d.BaseService.GetCtx(), query)
	if err != nil {
		return nil, nil, err
	}
	defer results.Close()

	mailboxMsgDataList := []*pb.MailboxItem{}
	find := false
	needDeleteKeyList := []string{}
	for result := range results.Next() {
		mailboxMsgData := &pb.MailboxItem{
			Key:     string(result.Key),
			Content: result.Value,
		}
		dmsgLog.Logger.Debugf("dmsgService->OnReadMailboxMsgRequest key:%v, value:%v", string(result.Key), string(result.Value))
		mailboxMsgDataList = append(mailboxMsgDataList, mailboxMsgData)
		needDeleteKeyList = append(needDeleteKeyList, string(result.Key))
		find = true
	}

	for _, needDeleteKey := range needDeleteKeyList {
		err := d.datastore.Delete(d.BaseService.GetCtx(), ds.NewKey(needDeleteKey))
		if err != nil {
			dmsgLog.Logger.Errorf("dmsgService->OnReadMailboxMsgRequest: delete msg happen err: %v", err)
		}
	}

	if !find {
		dmsgLog.Logger.Debug("dmsgService->OnReadMailboxMsgRequest: user msgs is empty")
	}
	dmsgLog.Logger.Debugf("dmsgService->OnReadMailboxMsgRequest end")
	return mailboxMsgDataList, nil, nil
}

func (d *DmsgService) OnCreateChannelRequest(requestProtoData protoreflect.ProtoMessage) (any, any, error) {
	dmsgLog.Logger.Debugf("dmsgService->OnCreateChannelRequest begin:\nrequestProtoData: %v", requestProtoData)

	request, ok := requestProtoData.(*pb.CreateChannelReq)
	if !ok {
		dmsgLog.Logger.Errorf("dmsgService->OnCreateChannelRequest: cannot convert to *pb.CreateMailboxReq")
		return nil, nil, fmt.Errorf("dmsgService->OnCreateChannelRequest: cannot convert to *pb.CreateMailboxReq")
	}
	channelKey := request.ChannelKey
	isAvailable := d.isAvailablePubChannel(channelKey)
	if !isAvailable {
		return nil, nil, errors.New("dmsgService->OnCreateChannelRequest: exceeded the maximum number of mailbox service")
	}
	pubChannelInfo := d.channelInfoList[channelKey]
	if pubChannelInfo != nil {
		dmsgLog.Logger.Errorf("dmsgService->OnCreateChannelRequest: pubChannelInfo already exist")
		retCode := &pb.RetCode{
			Code:   dmsgServiceCommon.PubChannelAlreadyExistCode,
			Result: "dmsgService->OnCreateChannelRequest: pubChannelInfo already exist",
		}
		return nil, retCode, nil
	}

	err := d.subscribePubChannel(channelKey)
	if err != nil {
		return nil, nil, err
	}

	dmsgLog.Logger.Debugf("dmsgService->OnCreateChannelRequest end")
	return nil, nil, nil
}

func (d *DmsgService) OnCreatePubChannelResponse(protoData protoreflect.ProtoMessage) (any, error) {
	return nil, nil
}

func (d *DmsgService) OnCustomStreamProtocolRequest(requestProtoData protoreflect.ProtoMessage) (any, any, error) {
	dmsgLog.Logger.Debugf("dmsgService->OnCustomStreamProtocolRequest begin:\nrequestProtoData: %v", requestProtoData)
	request, ok := requestProtoData.(*pb.CustomProtocolReq)
	if !ok {
		dmsgLog.Logger.Errorf("dmsgService->OnCustomStreamProtocolRequest: cannot convert to *pb.CustomContentReq")
		return nil, nil, fmt.Errorf("dmsgService->OnCustomStreamProtocolRequest: cannot convert to *pb.CustomContentReq")
	}

	customProtocolInfo := d.customStreamProtocolInfoList[request.PID]
	if customProtocolInfo == nil {
		dmsgLog.Logger.Errorf("dmsgService->OnCustomStreamProtocolRequest: customProtocolInfo is nil, request: %v", request)
		return nil, nil, fmt.Errorf("dmsgService->OnCustomStreamProtocolRequest: customProtocolInfo is nil, request: %v", request)
	}

	err := customProtocolInfo.Service.HandleRequest(request)
	if err != nil {
		return nil, nil, err
	}

	customStreamProtocolResponseParam := &serviceProtocol.CustomStreamProtocolResponseParam{
		PID:     request.PID,
		Service: customProtocolInfo.Service,
	}

	dmsgLog.Logger.Debugf("dmsgService->OnCustomStreamProtocolRequest end")
	return customStreamProtocolResponseParam, nil, nil
}

func (d *DmsgService) OnCustomStreamProtocolResponse(reqProtoData protoreflect.ProtoMessage, resProtoData protoreflect.ProtoMessage) (any, error) {
	return nil, nil
}

// pubsub
func (d *DmsgService) OnCustomPubsubProtocolRequest(requestProtoData protoreflect.ProtoMessage) (any, any, error) {
	dmsgLog.Logger.Debugf("dmsgService->OnCustomPubsubProtocolRequest begin:\nrequestProtoData: %v", requestProtoData)
	request, ok := requestProtoData.(*pb.CustomProtocolReq)
	if !ok {
		dmsgLog.Logger.Errorf("dmsgService->OnCustomPubsubProtocolRequest: cannot convert to *pb.CustomContentReq")
		return nil, nil, fmt.Errorf("dmsgService->OnCustomPubsubProtocolRequest: cannot convert to *pb.CustomContentReq")
	}

	customProtocolInfo := d.customPubsubProtocolInfoList[request.PID]
	if customProtocolInfo == nil {
		dmsgLog.Logger.Errorf("dmsgService->OnCustomPubsubProtocolRequest: customProtocolInfo is nil, request: %v", request)
		return nil, nil, fmt.Errorf("dmsgService->OnCustomPubsubProtocolRequest: customProtocolInfo is nil, request: %v", request)
	}

	err := customProtocolInfo.Service.HandleRequest(request)
	if err != nil {
		dmsgLog.Logger.Errorf("dmsgService->OnCustomPubsubProtocolRequest: HandleRequest error: %v", err)
		return nil, nil, err
	}

	responseParam := &serviceProtocol.CustomPubsubProtocolResponseParam{
		PID:     request.PID,
		Service: customProtocolInfo.Service,
	}

	dmsgLog.Logger.Debugf("dmsgService->OnCustomPubsubProtocolRequest end")
	return responseParam, nil, nil
}

func (d *DmsgService) OnCustomPubsubProtocolResponse(requestProtoData protoreflect.ProtoMessage, resProtoData protoreflect.ProtoMessage) (any, error) {
	return nil, nil
}

// PubsubProtocolCallback interface

func (d *DmsgService) OnSendMsgRequest(protoMsg protoreflect.ProtoMessage) (any, error) {
	request, ok := protoMsg.(*pb.SendMsgReq)
	if !ok {
		dmsgLog.Logger.Errorf("dmsgService->OnSendMsgRequest: cannot convert %v to *pb.SendMsgReq", protoMsg)
		return nil, fmt.Errorf("dmsgService->OnSendMsgRequest: cannot convert %v to *pb.SendMsgReq", protoMsg)
	}
	pubkey := request.BasicData.Pubkey
	pubsub := d.getDestUserPubsub(pubkey)
	if pubsub == nil {
		dmsgLog.Logger.Errorf("dmsgService->OnSendMsgRequest: public key %s is not exist", pubkey)
		return nil, fmt.Errorf("dmsgService->OnSendMsgRequest: public key %s is not exist", pubkey)
	}
	pubsub.LastReciveTimestamp = time.Now().Unix()
	d.saveUserMsg(protoMsg)
	return nil, nil
}

func (d *DmsgService) PublishProtocol(ctx context.Context, userPubkey string, protocolID pb.PID, protocolData []byte) error {
	userPubsub := d.getDestUserPubsub(userPubkey)
	if userPubsub == nil {
		dmsgLog.Logger.Errorf("dmsgService->PublishProtocol: no find user pubic key %s for pubsub", userPubkey)
		return fmt.Errorf("dmsgService->PublishProtocol: no find user pubic key %s for pubsub", userPubkey)
	}
	pubsubBuf := new(bytes.Buffer)
	err := binary.Write(pubsubBuf, binary.LittleEndian, protocolID)
	if err != nil {
		return err
	}
	err = binary.Write(pubsubBuf, binary.LittleEndian, protocolData)
	if err != nil {
		return err
	}
	pubsubData := pubsubBuf.Bytes()
	if err := userPubsub.Topic.Publish(ctx, pubsubData); err != nil {
		dmsgLog.Logger.Errorf("dmsgService->PublishProtocol: publish protocol err: %v", err)
		return fmt.Errorf("dmsgService->PublishProtocol: publish protocol err: %v", err)
	}
	return nil
}

// custom stream protocol
func (d *DmsgService) RegistCustomStreamProtocol(service customProtocol.CustomStreamProtocolService) error {
	customProtocolID := service.GetProtocolID()
	if d.customStreamProtocolInfoList[customProtocolID] != nil {
		dmsgLog.Logger.Errorf("dmsgService->RegistCustomStreamProtocol: protocol %s is already exist", customProtocolID)
		return fmt.Errorf("dmsgService->RegistCustomStreamProtocol: protocol %s is already exist", customProtocolID)
	}
	d.customStreamProtocolInfoList[customProtocolID] = &dmsgServiceCommon.CustomStreamProtocolInfo{
		Protocol: serviceProtocol.NewCustomStreamProtocol(d.BaseService.GetCtx(), d.BaseService.GetHost(), customProtocolID, d, d),
		Service:  service,
	}
	service.SetCtx(d.BaseService.GetCtx())
	return nil
}

func (d *DmsgService) UnregistCustomStreamProtocol(callback customProtocol.CustomStreamProtocolService) error {
	customProtocolID := callback.GetProtocolID()
	if d.customStreamProtocolInfoList[customProtocolID] == nil {
		dmsgLog.Logger.Warnf("dmsgService->UnregistCustomStreamProtocol: protocol %s is not exist", customProtocolID)
		return nil
	}
	d.customStreamProtocolInfoList[customProtocolID] = &dmsgServiceCommon.CustomStreamProtocolInfo{
		Protocol: serviceProtocol.NewCustomStreamProtocol(d.BaseService.GetCtx(), d.BaseService.GetHost(), customProtocolID, d, d),
		Service:  callback,
	}
	return nil
}

// custom pubsub protocol
func (d *DmsgService) getCustomProtocolPubsub(pubkey string) *dmsgServiceCommon.CustomProtocolPubsub {
	return d.customProtocolPubsubList[pubkey]
}

func (d *DmsgService) publishCustomProtocol(pubkey string) error {
	pubsub := d.getCustomProtocolPubsub(pubkey)
	if pubsub != nil {
		dmsgLog.Logger.Errorf("dmsgService->publishCustomProtocol: user public key(%s) pubsub already exist", pubkey)
		return fmt.Errorf("dmsgService->publishCustomProtocol: user public key(%s) pubsub already exist", pubkey)
	}
	err := d.subscribeCustomProtocol(pubkey)
	if err != nil {
		return err
	}
	pubsub = d.getCustomProtocolPubsub(pubkey)
	go d.readCustomProtocolPubsub(&pubsub.UserPubsub)
	return nil
}

func (d *DmsgService) unpublishCustomProtocol(pubkey string) error {
	pubsub := d.getCustomProtocolPubsub(pubkey)
	if pubsub == nil {
		return nil
	}
	err := d.unsubscribeCustomProtocol(pubkey)
	if err != nil {
		return err
	}
	return nil
}

func (d *DmsgService) unsubscribeCustomProtocol(pubKey string) error {
	pubsub := d.customProtocolPubsubList[pubKey]
	if pubsub == nil {
		dmsgLog.Logger.Errorf("dmsgService->unSubscribeCustomProtocol: no find pubKey %s for pubsub", pubKey)
		return fmt.Errorf("dmsgService->unSubscribeCustomProtocol: no find pubKey %s for pubsub", pubKey)
	}
	pubsub.CancelFunc()
	pubsub.Subscription.Cancel()
	err := pubsub.Topic.Close()
	if err != nil {
		dmsgLog.Logger.Warnf("dmsgService->unSubscribeCustomProtocol: userTopic.Close error: %v", err)
	}
	delete(d.customProtocolPubsubList, pubKey)
	return nil
}

func (d *DmsgService) subscribeCustomProtocol(pubKey string) error {
	var err error
	topic, err := d.Pubsub.Join(pubKey)
	if err != nil {
		return err
	}
	subscribe, err := topic.Subscribe()
	if err != nil {
		return err
	}
	// go d.BaseService.DiscoverRendezvousPeers()

	d.customProtocolPubsubList[pubKey] = &dmsgServiceCommon.CustomProtocolPubsub{
		UserPubsub: dmsgServiceCommon.UserPubsub{
			Topic:        topic,
			Subscription: subscribe,
		},
	}
	return nil
}

func (d *DmsgService) unsubscribeCustomProtocolList() error {
	for pubkey := range d.customProtocolPubsubList {
		d.unsubscribeCustomProtocol(pubkey)
	}
	return nil
}

func (d *DmsgService) readCustomProtocolPubsub(pubsub *dmsgServiceCommon.UserPubsub) {
	d.readProtocolPubsub(pubsub)
}

func (d *DmsgService) RegistCustomPubsubProtocol(service customProtocol.CustomStreamProtocolService, pubkey string) error {
	customProtocolID := service.GetProtocolID()
	if d.customPubsubProtocolInfoList[customProtocolID] != nil {
		dmsgLog.Logger.Errorf("dmsgService->RegistCustomPubsubProtocol: customProtocolID %s is already exist", customProtocolID)
		return fmt.Errorf("dmsgService->RegistCustomPubsubProtocol: customProtocolID %s is already exist", customProtocolID)
	}
	d.customPubsubProtocolInfoList[customProtocolID] = &dmsgServiceCommon.CustomPubsubProtocolInfo{
		Protocol: serviceProtocol.NewCustomPubsubProtocol(d.BaseService.GetCtx(), d.BaseService.GetHost(), customProtocolID, d, d),
		Service:  service,
	}
	service.SetCtx(d.BaseService.GetCtx())
	// TODO
	// err := d.PublishCustomPubsubProtocol(pubkey)
	// if err != nil {
	// 	dmsgLog.Logger.Errorf("dmsgService->RegistCustomPubsubProtocol: publish dest user err: %v", err)
	// 	return err
	// }
	return nil
}

func (d *DmsgService) UnregistCustomPubsubProtocol(callback customProtocol.CustomStreamProtocolService) error {
	customProtocolID := callback.GetProtocolID()
	if d.customPubsubProtocolInfoList[customProtocolID] == nil {
		dmsgLog.Logger.Warnf("dmsgService->UnregistCustomStreamProtocol: protocol %s is not exist", customProtocolID)
		return nil
	}
	d.customPubsubProtocolInfoList[customProtocolID] = &dmsgServiceCommon.CustomPubsubProtocolInfo{
		Protocol: serviceProtocol.NewCustomPubsubProtocol(d.BaseService.GetCtx(), d.BaseService.GetHost(), customProtocolID, d, d),
		Service:  callback,
	}
	return nil
}

// user
func (d *DmsgService) subscribeUser(pubkey string, getSig dmsgKey.GetSigCallback) error {
	dmsgLog.Logger.Debugf("DmsgService->subscribeUser begin\npubkey: %s", pubkey)
	if d.user != nil {
		dmsgLog.Logger.Errorf("DmsgService->subscribeUser: user isn't nil")
		return fmt.Errorf("DmsgService->subscribeUser: user isn't nil")
	}

	if d.destUserList[pubkey] != nil {
		dmsgLog.Logger.Errorf("DmsgService->subscribeUser: pubkey is already exist in destUserList")
		return fmt.Errorf("DmsgService->subscribeUser: pubkey is already exist in destUserList")
	}

	user, err := dmsgUser.NewUser(d.BaseService.GetCtx(), d.Pubsub, pubkey, getSig)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->subscribeUser: NewUser error: %v", err)
		return err
	}

	d.user = &dmsgUser.LightUser{
		User:          *user,
		ServicePeerID: "",
	}

	// go d.BaseService.DiscoverRendezvousPeers()
	go d.HandleProtocolWithPubsub(&d.user.User)

	dmsgLog.Logger.Debugf("DmsgService->subscribeUser end")
	return nil
}

func (d *DmsgService) unsubscribeUser() error {
	dmsgLog.Logger.Debugf("DmsgService->unsubscribeUser begin")
	if d.user == nil {
		dmsgLog.Logger.Errorf("DmsgService->unsubscribeUser: userPubkey is not exist in destUserInfoList")
		return fmt.Errorf("DmsgService->unsubscribeUser: userPubkey is not exist in destUserInfoList")
	}
	d.user.Close()
	d.user = nil
	dmsgLog.Logger.Debugf("DmsgService->unsubscribeUser end")
	return nil
}

func daysBetween(start, end int64) int {
	startTime := time.Unix(start, 0)
	endTime := time.Unix(end, 0)
	return int(endTime.Sub(startTime).Hours() / 24)
}
