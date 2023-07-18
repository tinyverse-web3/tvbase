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
	dmsgLog "github.com/tinyverse-web3/tvbase/dmsg/common/log"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol"
	customProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol/custom"
	dmsgServiceCommon "github.com/tinyverse-web3/tvbase/dmsg/service/common"
	serviceProtocol "github.com/tinyverse-web3/tvbase/dmsg/service/protocol"
	tvCrypto "github.com/tinyverse-web3/tvutil/crypto"
	keyUtil "github.com/tinyverse-web3/tvutil/key"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type ProtocolProxy struct {
	createMailboxProtocol *dmsgServiceCommon.StreamProtocol
	releaseMailboxPrtocol *dmsgServiceCommon.StreamProtocol
	readMailboxMsgPrtocol *dmsgServiceCommon.StreamProtocol
	seekMailboxProtocol   *dmsgServiceCommon.PubsubProtocol
	sendMsgPrtocol        *serviceProtocol.SendMsgProtocol
}

type DmsgService struct {
	dmsg.DmsgService
	ProtocolProxy
	datastore                    db.Datastore
	curSrcUserInfo               *dmsgServiceCommon.UserInfo
	destUserPubsubs              map[string]*dmsgServiceCommon.DestUserPubsub
	customProtocolPubsubs        map[string]*dmsgServiceCommon.CustomProtocolPubsub
	protocolReqSubscribes        map[pb.ProtocolID]protocol.ReqSubscribe
	protocolResSubscribes        map[pb.ProtocolID]protocol.ResSubscribe
	customStreamProtocolInfoList map[string]*dmsgServiceCommon.CustomStreamProtocolInfo
	customPubsubProtocolInfoList map[string]*dmsgServiceCommon.CustomPubsubProtocolInfo
	stopCheckDestPubsubs         chan bool
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

	// pubsub protocol
	d.seekMailboxProtocol = serviceProtocol.NewSeekMailboxProtocol(d.BaseService.GetHost(), d, d)

	// sendMsgProtocol, special pubsub protocol
	d.sendMsgPrtocol = serviceProtocol.NewSendMsgProtocol(d.BaseService.GetHost(), d, d)

	d.destUserPubsubs = make(map[string]*dmsgServiceCommon.DestUserPubsub)
	d.customProtocolPubsubs = make(map[string]*dmsgServiceCommon.CustomProtocolPubsub)

	d.protocolReqSubscribes = make(map[pb.ProtocolID]protocol.ReqSubscribe)
	d.protocolResSubscribes = make(map[pb.ProtocolID]protocol.ResSubscribe)

	d.customStreamProtocolInfoList = make(map[string]*dmsgServiceCommon.CustomStreamProtocolInfo)
	d.customPubsubProtocolInfoList = make(map[string]*dmsgServiceCommon.CustomPubsubProtocolInfo)

	peerId := d.BaseService.GetHost().ID().String()
	priKey, pubKey, err := keyUtil.GenerateEcdsaKey(peerId)
	if err != nil {
		dmsgLog.Logger.Errorf("dmsgService->Init: generate priKey error %v", err)
		return err
	}
	priKeyData, err := keyUtil.ECDSAPrivateKeyToProtoBuf(priKey)
	if err != nil {
		dmsgLog.Logger.Errorf("dmsgService->Init: generate priKey_porto error %v", err)
		return err
	}
	priKeyHex := keyUtil.TranslateKeyProtoBufToString(priKeyData)

	pubKeyData, err := keyUtil.ECDSAPublicKeyToProtoBuf(pubKey)
	if err != nil {
		dmsgLog.Logger.Errorf("dmsgService->Init: generate pubKey_porto error %v", err)
		return err
	}
	pubKeyHex := keyUtil.TranslateKeyProtoBufToString(pubKeyData)

	d.curSrcUserInfo = &dmsgServiceCommon.UserInfo{
		UserKey: &dmsgServiceCommon.UserKey{
			PubKeyHex: pubKeyHex,
			PriKeyHex: priKeyHex,
			PubKey:    pubKey,
			PriKey:    priKey,
		},
	}

	d.stopCheckDestPubsubs = make(chan bool)
	return nil
}

func (d *DmsgService) GetCurSrcUserPubKeyHex() string {
	return d.curSrcUserInfo.UserKey.PubKeyHex
}

func (d *DmsgService) GetCurSrcUserSign(protoData []byte) ([]byte, error) {
	sign, err := tvCrypto.SignDataByEcdsa(d.curSrcUserInfo.UserKey.PriKey, protoData)
	if err != nil {
		dmsgLog.Logger.Errorf("GetCurUserSign: %v", err)
		return sign, nil
	}
	return sign, nil
}

func (d *DmsgService) getMsgPrefix(userPubkey string) string {
	return dmsg.MsgPrefix + userPubkey
}

func (d *DmsgService) readProtocolPubsub(pubsub *dmsgServiceCommon.CommonPubsub) {
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
			reqSubscribe.OnRequest(m, contentData)
			continue
		} else {
			dmsgLog.Logger.Warnf("dmsgService->readDestUserPubsub: no find protocolId(%d) for reqSubscribe", protocolID)
		}
		// no protocol for resSubScribe in service
		resSubScribe := d.PubsubProtocolResSubscribes[protocolID]
		if resSubScribe != nil {
			resSubScribe.OnResponse(m, contentData)
			continue
		} else {
			dmsgLog.Logger.Warnf("dmsgService->readDestUserPubsub: no find protocolId(%d) for resSubscribe", protocolID)
		}
	}
}

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

func (d *DmsgService) checkDestUserPubs(done chan bool) {
	go func() {
		ticker := time.NewTicker(3 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				cfg := d.BaseService.GetConfig()
				for userPubkey, pubsub := range d.destUserPubsubs {
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
				continue
			case <-d.BaseService.GetCtx().Done():
				return
			}
		}
	}()
}

func (d *DmsgService) readDestUserPubsub(pubsub *dmsgServiceCommon.DestUserPubsub) {
	d.readProtocolPubsub(&pubsub.CommonPubsub)
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

	d.destUserPubsubs[userPubKey] = &dmsgServiceCommon.DestUserPubsub{
		CommonPubsub: dmsgServiceCommon.CommonPubsub{
			Topic:        userTopic,
			Subscription: userSub,
		},
		MsgRWMutex:          sync.RWMutex{},
		LastReciveTimestamp: time.Now().Unix(),
	}
	return nil
}

func (d *DmsgService) unSubscribeDestUser(userPubKey string) error {
	pubsub := d.destUserPubsubs[userPubKey]
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
	delete(d.destUserPubsubs, userPubKey)
	return nil
}

func (d *DmsgService) unSubscribeDestUsers() error {
	for userPubKey := range d.destUserPubsubs {
		d.unSubscribeDestUser(userPubKey)
	}
	return nil
}

func (d *DmsgService) getDestUserPubsub(destUserPubsub string) *dmsgServiceCommon.DestUserPubsub {
	return d.destUserPubsubs[destUserPubsub]
}

func (d *DmsgService) saveUserMsg(protoMsg protoreflect.ProtoMessage, msgContent []byte) error {
	sendMsgReq, ok := protoMsg.(*pb.SendMsgReq)
	if !ok {
		dmsgLog.Logger.Errorf("dmsgService->saveUserMsg: cannot convert %v to *pb.SendMsgReq", protoMsg)
		return fmt.Errorf("dmsgService->saveUserMsg: cannot convert %v to *pb.SendMsgReq", protoMsg)
	}

	pubkey := sendMsgReq.BasicData.DestPubkey
	pubsub := d.getDestUserPubsub(pubkey)
	if pubsub == nil {
		dmsgLog.Logger.Errorf("dmsgService->saveUserMsg: cannot find src user pubsub for %v", pubkey)
		return fmt.Errorf("dmsgService->saveUserMsg: cannot find src user pubsub for %v", pubkey)
	}

	pubsub.MsgRWMutex.RLock()
	defer pubsub.MsgRWMutex.RUnlock()

	key := d.GetFullFromMsgPrefix(sendMsgReq)
	err := d.datastore.Put(d.BaseService.GetCtx(), ds.NewKey(key), sendMsgReq.MsgContent)
	if err != nil {
		return err
	}

	return err
}

func (d *DmsgService) isAvailableMailbox(userPubKey string) bool {
	destUserCount := len(d.destUserPubsubs)
	cfg := d.BaseService.GetConfig()
	if destUserCount >= cfg.DMsg.MaxMailboxPubsubCount {
		dmsgLog.Logger.Warnf("dmsgService->isAvailableMailbox: exceeded the maximum number of mailbox services, current destUserCount:%v", destUserCount)
		return false
	}
	return true
}

// for sdk
func (d *DmsgService) Start() error {
	err := d.DmsgService.Start()
	if err != nil {
		return err
	}
	d.checkDestUserPubs(d.stopCheckDestPubsubs)
	return nil
}

func (d *DmsgService) Stop() error {
	err := d.DmsgService.Stop()
	if err != nil {
		return err
	}

	d.unSubscribeDestUsers()
	d.stopCheckDestPubsubs <- true
	return nil
}

func (d *DmsgService) GetDestUserPubsub(publicKey string) *dmsgServiceCommon.DestUserPubsub {
	return d.destUserPubsubs[publicKey]
}

// StreamProtocolCallback interface
func (d *DmsgService) OnCreateMailboxRequest(protoData protoreflect.ProtoMessage) (interface{}, error) {
	request, ok := protoData.(*pb.CreateMailboxReq)
	if !ok {
		dmsgLog.Logger.Errorf("dmsgService->OnCreateMailboxRequest: cannot convert %v to *pb.CreateMailboxReq", protoData)
		return nil, fmt.Errorf("dmsgService->OnCreateMailboxRequest: cannot convert %v to *pb.CreateMailboxReq", protoData)
	}
	isAvailable := d.isAvailableMailbox(request.BasicData.DestPubkey)
	if !isAvailable {
		return nil, errors.New(dmsgServiceCommon.MailboxLimitErr)
	}
	err := d.publishDestUser(request.BasicData.DestPubkey)
	if err != nil {
		dmsgLog.Logger.Warnf("dmsgService->OnCreateMailboxResponse: destPubkey %s already exist", request.BasicData.DestPubkey)
	}
	return nil, nil
}

func (d *DmsgService) OnCreateMailboxResponse(protoData protoreflect.ProtoMessage) (interface{}, error) {
	response, ok := protoData.(*pb.CreateMailboxRes)
	if !ok {
		dmsgLog.Logger.Errorf("dmsgService->OnCreateMailboxResponse: cannot convert %v to *pb.CreateMailboxRes", protoData)
		return nil, fmt.Errorf("dmsgService->OnCreateMailboxResponse: cannot convert %v to *pb.CreateMailboxRes", protoData)
	}

	pubsub := d.getDestUserPubsub(response.BasicData.DestPubkey)
	if pubsub != nil {
		dmsgLog.Logger.Warnf("dmsgService->OnCreateMailboxResponse: destPubkey %s already exist", response.BasicData.DestPubkey)
		response.RetCode = &pb.RetCode{
			Code:   dmsgServiceCommon.MailboxAlreadyExistCode,
			Result: dmsgServiceCommon.MailboxAlreadyExistErr,
		}
	}
	return nil, nil
}

func (d *DmsgService) OnReleaseMailboxRequest(protoData protoreflect.ProtoMessage) (interface{}, error) {
	request, ok := protoData.(*pb.ReleaseMailboxReq)
	if !ok {
		dmsgLog.Logger.Errorf("dmsgService->OnReleaseMailboxRequest: cannot convert %v to *pb.ReleaseMailboxReq", protoData)
		return nil, fmt.Errorf("dmsgService->OnReleaseMailboxRequest: cannot convert %v to *pb.ReleaseMailboxReq", protoData)
	}

	err := d.unPublishDestUser(request.BasicData.DestPubkey)
	if err != nil {
		return nil, err
	}
	return nil, nil
}
func (d *DmsgService) OnReadMailboxMsgRequest(protoData protoreflect.ProtoMessage) (interface{}, error) {
	request, ok := protoData.(*pb.ReadMailboxMsgReq)
	if !ok {
		dmsgLog.Logger.Errorf("dmsgService->OnReadMailboxMsgRequest: cannot convert %v to *pb.ReadMailboxMsgReq", protoData)
		return nil, fmt.Errorf("dmsgService->OnReadMailboxMsgRequest: cannot convert %v to *pb.ReadMailboxMsgReq", protoData)
	}

	pubkey := request.BasicData.DestPubkey
	pubsub := d.getDestUserPubsub(pubkey)
	if pubsub == nil {
		dmsgLog.Logger.Errorf("dmsgService->OnReadMailboxMsgRequest: cannot find src user pubsub for %s", pubkey)
		return nil, fmt.Errorf("dmsgService->OnReadMailboxMsgRequest: cannot find src user pubsub for %s", pubkey)
	}
	pubsub.MsgRWMutex.RLock()
	defer pubsub.MsgRWMutex.RUnlock()

	var query = query.Query{
		Prefix: d.getMsgPrefix(pubkey),
	}
	results, err := d.datastore.Query(d.BaseService.GetCtx(), query)
	if err != nil {
		return nil, err
	}
	defer results.Close()

	mailboxMsgDataList := []*pb.MailboxMsgData{}
	find := false
	needDeleteKeyList := []string{}
	for result := range results.Next() {
		mailboxMsgData := &pb.MailboxMsgData{
			Key:        string(result.Key),
			MsgContent: result.Value,
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
	return mailboxMsgDataList, nil
}

func (d *DmsgService) OnCustomStreamProtocolRequest(protoData protoreflect.ProtoMessage) (interface{}, error) {
	request, ok := protoData.(*pb.CustomProtocolReq)
	if !ok {
		dmsgLog.Logger.Errorf("dmsgService->OnCustomStreamProtocolRequest: cannot convert %v to *pb.CustomContentReq", protoData)
		return nil, fmt.Errorf("dmsgService->OnCustomStreamProtocolRequest: cannot convert %v to *pb.CustomContentReq", protoData)
	}

	customProtocolInfo := d.customStreamProtocolInfoList[request.CustomProtocolID]
	if customProtocolInfo == nil {
		dmsgLog.Logger.Errorf("dmsgService->OnCustomStreamProtocolRequest: customProtocolInfo is nil, request: %v", request)
		return nil, fmt.Errorf("dmsgService->OnCustomStreamProtocolRequest: customProtocolInfo is nil, request: %v", request)
	}

	if customProtocolInfo.Service == nil {
		dmsgLog.Logger.Errorf("dmsgService->OnCustomStreamProtocolRequest: callback is nil")
		return nil, fmt.Errorf("dmsgService->OnCustomStreamProtocolRequest: callback is nil")
	}
	customProtocolInfo.Service.HandleRequest(request)
	return request, nil
}

func (d *DmsgService) OnCustomStreamProtocolResponse(reqProtoData protoreflect.ProtoMessage, resProtoData protoreflect.ProtoMessage) (interface{}, error) {
	response, ok := resProtoData.(*pb.CustomProtocolRes)
	if !ok {
		dmsgLog.Logger.Errorf("dmsgService->OnCustomStreamProtocolResponse: cannot convert %v to *pb.CustomContentRes", resProtoData)
		return nil, fmt.Errorf("dmsgService->OnCustomStreamProtocolResponse: cannot convert %v to *pb.CustomContentRes", resProtoData)
	}
	customProtocolInfo := d.customStreamProtocolInfoList[response.CustomProtocolID]
	if customProtocolInfo == nil {
		dmsgLog.Logger.Errorf("dmsgService->OnCustomStreamProtocolResponse: customProtocolInfo is nil, response: %v", response)
		return nil, fmt.Errorf("dmsgService->OnCustomStreamProtocolResponse: customProtocolInfo is nil, response: %v", response)
	}
	if customProtocolInfo.Service == nil {
		dmsgLog.Logger.Errorf("dmsgService->OnCustomStreamProtocolResponse: callback is nil")
		return nil, fmt.Errorf("dmsgService->OnCustomStreamProtocolResponse: callback is nil")
	}

	request, ok := reqProtoData.(*pb.CustomProtocolReq)
	if !ok {
		dmsgLog.Logger.Errorf("dmsgService->OnCustomStreamProtocolResponse: cannot convert %v to *pb.CustomContentRes", reqProtoData)
		return nil, fmt.Errorf("dmsgService->OnCustomStreamProtocolResponse: cannot convert %v to *pb.CustomContentRes", reqProtoData)
	}

	err := customProtocolInfo.Service.HandleResponse(request, response)
	if err != nil {
		dmsgLog.Logger.Errorf("dmsgService->OnCustomStreamProtocolResponse: callback happen err: %v", err)
		return nil, err
	}
	return nil, nil
}

func (d *DmsgService) OnCustomPubsubProtocolRequest(protoData protoreflect.ProtoMessage) (interface{}, error) {
	request, ok := protoData.(*pb.CustomProtocolReq)
	if !ok {
		dmsgLog.Logger.Errorf("dmsgService->OnCustomPubsubProtocolRequest: cannot convert %v to *pb.CustomContentReq", protoData)
		return nil, fmt.Errorf("dmsgService->OnCustomPubsubProtocolRequest: cannot convert %v to *pb.CustomContentReq", protoData)
	}

	customPubsubProtocolInfo := d.customPubsubProtocolInfoList[request.CustomProtocolID]
	if customPubsubProtocolInfo == nil {
		dmsgLog.Logger.Errorf("dmsgService->OnCustomPubsubProtocolRequest: customProtocolInfo is nil, request: %v", request)
		return nil, fmt.Errorf("dmsgService->OnCustomPubsubProtocolRequest: customProtocolInfo is nil, request: %v", request)
	}

	if customPubsubProtocolInfo.Service == nil {
		dmsgLog.Logger.Errorf("dmsgService->OnCustomPubsubProtocolRequest: Service is nil")
		return nil, fmt.Errorf("dmsgService->OnCustomPubsubProtocolRequest: Service is nil")
	}
	customPubsubProtocolInfo.Service.HandleRequest(request)
	return request, nil
}

func (d *DmsgService) OnCustomPubsubProtocolResponse(reqProtoData protoreflect.ProtoMessage, resProtoData protoreflect.ProtoMessage) (interface{}, error) {
	response, ok := resProtoData.(*pb.CustomProtocolRes)
	if !ok {
		dmsgLog.Logger.Errorf("dmsgService->OnCustomPubsubProtocolResponse: cannot convert %v to *pb.CustomContentRes", resProtoData)
		return nil, fmt.Errorf("dmsgService->OnCustomPubsubProtocolResponse: cannot convert %v to *pb.CustomContentRes", resProtoData)
	}
	customPubsubProtocolInfo := d.customPubsubProtocolInfoList[response.CustomProtocolID]
	if customPubsubProtocolInfo == nil {
		dmsgLog.Logger.Errorf("dmsgService->OnCustomPubsubProtocolResponse: customProtocolInfo is nil, response: %v", response)
		return nil, fmt.Errorf("dmsgService->OnCustomPubsubProtocolResponse: customProtocolInfo is nil, response: %v", response)
	}
	if customPubsubProtocolInfo.Service == nil {
		dmsgLog.Logger.Errorf("dmsgService->OnCustomPubsubProtocolResponse: Service is nil")
		return nil, fmt.Errorf("dmsgService->OnCustomPubsubProtocolResponse: Service is nil")
	}

	request, ok := reqProtoData.(*pb.CustomProtocolReq)
	if !ok {
		dmsgLog.Logger.Errorf("dmsgService->OnCustomPubsubProtocolResponse: cannot convert %v to *pb.CustomContentRes", reqProtoData)
		return nil, fmt.Errorf("dmsgService->OnCustomPubsubProtocolResponse: cannot convert %v to *pb.CustomContentRes", reqProtoData)
	}

	err := customPubsubProtocolInfo.Service.HandleResponse(request, response)
	if err != nil {
		dmsgLog.Logger.Errorf("dmsgService->OnCustomPubsubProtocolResponse: HandleResponse happen err: %v", err)
		return nil, err
	}
	return nil, nil
}

// PubsubProtocolCallback interface
func (d *DmsgService) OnSeekMailboxRequest(protoData protoreflect.ProtoMessage) (interface{}, error) {
	request, ok := protoData.(*pb.SeekMailboxReq)
	if !ok {
		dmsgLog.Logger.Errorf("dmsgService->OnSeekMailboxRequest: cannot convert %v to *pb.SeekMailboxReq", protoData)
		return nil, fmt.Errorf("dmsgService->OnSeekMailboxRequest: cannot convert %v to *pb.SeekMailboxReq", protoData)
	}
	pubsub := d.destUserPubsubs[request.BasicData.DestPubkey]
	if pubsub == nil {
		dmsgLog.Logger.Errorf("dmsgService->OnSeekMailboxRequest: mailbox not exist for pubic key:%v", request.BasicData.DestPubkey)
		return nil, fmt.Errorf("dmsgService->OnSeekMailboxRequest: mailbox not exist for pubic key:%v", request.BasicData.DestPubkey)
	}
	return nil, nil
}

func (d *DmsgService) OnHandleSendMsgRequest(protoMsg protoreflect.ProtoMessage, protoData []byte) (interface{}, error) {
	request, ok := protoMsg.(*pb.SendMsgReq)
	if !ok {
		dmsgLog.Logger.Errorf("dmsgService->OnHandleSendMsgRequest: cannot convert %v to *pb.SendMsgReq", protoMsg)
		return nil, fmt.Errorf("dmsgService->OnHandleSendMsgRequest: cannot convert %v to *pb.SendMsgReq", protoMsg)
	}
	pubkey := request.BasicData.DestPubkey
	pubsub := d.getDestUserPubsub(pubkey)
	if pubsub == nil {
		dmsgLog.Logger.Errorf("dmsgService->OnHandleSendMsgRequest: public key %s is not exist", pubkey)
		return nil, fmt.Errorf("dmsgService->OnHandleSendMsgRequest: public key %s is not exist", pubkey)
	}
	pubsub.LastReciveTimestamp = time.Now().Unix()
	d.saveUserMsg(protoMsg, protoData)
	return nil, nil
}

func (d *DmsgService) PublishProtocol(protocolID pb.ProtocolID, userPubkey string, protocolData []byte) error {
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
	if err := userPubsub.Topic.Publish(d.BaseService.GetCtx(), pubsubData); err != nil {
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
	return d.customProtocolPubsubs[pubkey]
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
	go d.readCustomProtocolPubsub(&pubsub.CommonPubsub)
	return nil
}

func (d *DmsgService) unPublishCustomProtocol(userPubkey string) error {
	pubsub := d.getCustomProtocolPubsub(userPubkey)
	if pubsub == nil {
		return nil
	}
	err := d.unSubscribeCustomProtocol(userPubkey)
	if err != nil {
		return err
	}
	return nil
}

func (d *DmsgService) unSubscribeCustomProtocol(pubKey string) error {
	pubsub := d.customProtocolPubsubs[pubKey]
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
	delete(d.customProtocolPubsubs, pubKey)
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

	d.customProtocolPubsubs[pubKey] = &dmsgServiceCommon.CustomProtocolPubsub{
		CommonPubsub: dmsgServiceCommon.CommonPubsub{
			Topic:        topic,
			Subscription: subscribe,
		},
	}
	return nil
}

func (d *DmsgService) unSubscribeCustomProtocols() error {
	for pubkey := range d.customProtocolPubsubs {
		d.unSubscribeCustomProtocol(pubkey)
	}
	return nil
}

func (d *DmsgService) readCustomProtocolPubsub(pubsub *dmsgServiceCommon.CommonPubsub) {
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

func daysBetween(start, end int64) int {
	startTime := time.Unix(start, 0)
	endTime := time.Unix(end, 0)
	return int(endTime.Sub(startTime).Hours() / 24)
}
