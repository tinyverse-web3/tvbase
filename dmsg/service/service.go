package service

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"time"

	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	tvCommon "github.com/tinyverse-web3/tvbase/common"
	"github.com/tinyverse-web3/tvbase/common/db"
	tvLog "github.com/tinyverse-web3/tvbase/common/log"
	"github.com/tinyverse-web3/tvbase/dmsg"
	dmsgLog "github.com/tinyverse-web3/tvbase/dmsg/common/log"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol"
	customProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol/custom"
	dmsgServiceCommon "github.com/tinyverse-web3/tvbase/dmsg/service/common"
	serviceProtocol "github.com/tinyverse-web3/tvbase/dmsg/service/protocol"
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
	Datastore       db.Datastore
	destUserPubsubs map[string]*dmsgServiceCommon.DestUserPubsub
	// service
	protocolReqSubscribes        map[pb.ProtocolID]protocol.ReqSubscribe
	protocolResSubscribes        map[pb.ProtocolID]protocol.ResSubscribe
	customStreamProtocolInfoList map[string]*dmsgServiceCommon.CustomProtocolInfo
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
	d.Datastore, err = db.CreateDataStore(cfg.DMsg.DatastorePath, cfg.Mode)
	if err != nil {
		return err
	}

	// stream protocol
	d.createMailboxProtocol = serviceProtocol.NewCreateMailboxProtocol(d.BaseService.GetCtx(), d.BaseService.GetHost(), d)
	d.releaseMailboxPrtocol = serviceProtocol.NewReleaseMailboxProtocol(d.BaseService.GetCtx(), d.BaseService.GetHost(), d)
	d.readMailboxMsgPrtocol = serviceProtocol.NewReadMailboxMsgProtocol(d.BaseService.GetCtx(), d.BaseService.GetHost(), d)

	// pubsub protocol
	d.seekMailboxProtocol = serviceProtocol.NewSeekMailboxProtocol(d.BaseService.GetHost(), d, d)

	// sendMsgProtocol, special pubsub protocol
	d.sendMsgPrtocol = serviceProtocol.NewSendMsgProtocol(d.BaseService.GetHost(), d, d)

	d.destUserPubsubs = make(map[string]*dmsgServiceCommon.DestUserPubsub)

	d.protocolReqSubscribes = make(map[pb.ProtocolID]protocol.ReqSubscribe)
	d.protocolResSubscribes = make(map[pb.ProtocolID]protocol.ResSubscribe)

	d.customStreamProtocolInfoList = make(map[string]*dmsgServiceCommon.CustomProtocolInfo)
	return nil
}

func (d *DmsgService) getMsgPrefix(srcUserPubkey string) string {
	return dmsg.MsgPrefix + srcUserPubkey
}

func (d *DmsgService) publishDestUser(pubkey string) error {
	pubsub := d.getDestUserPubsub(pubkey)
	if pubsub != nil {
		dmsgLog.Logger.Errorf("serviceDmsgService->publishDestUser: user public key(%s) pubsub already exist", pubkey)
		return fmt.Errorf("serviceDmsgService->publishDestUser: user public key(%s) pubsub already exist", pubkey)
	}
	err := d.subscribeDestUser(pubkey)
	if err != nil {
		return err
	}
	pubsub = d.getDestUserPubsub(pubkey)
	go d.readDestUserPubsub(pubkey, pubsub)
	return nil
}

func (d *DmsgService) unPublishDestUser(destPubkey string) error {
	pubsub := d.getDestUserPubsub(destPubkey)
	if pubsub == nil {
		return nil
	}
	err := d.unSubscribeDestUser(destPubkey)
	if err != nil {
		return err
	}
	return nil
}

func (d *DmsgService) readDestUserPubsub(pubkey string, pubsub *dmsgServiceCommon.DestUserPubsub) {
	for {
		days := daysBetween(pubsub.LastReciveTimestamp, time.Now().Unix())
		// delete mailbox msg in datastore and cancel mailbox subscribe when days is over, default day is 30
		cfg := d.BaseService.GetConfig()
		if days >= cfg.DMsg.KeepMailboxMsgDay {
			var query = query.Query{
				Prefix:   d.getMsgPrefix(pubkey),
				KeysOnly: true,
			}
			results, err := d.Datastore.Query(d.BaseService.GetCtx(), query)
			if err != nil {
				dmsgLog.Logger.Errorf("serviceDmsgService->readDestUserPubsub: query error: %v", err)
			}

			for result := range results.Next() {
				d.Datastore.Delete(d.BaseService.GetCtx(), ds.NewKey(result.Key))
				dmsgLog.Logger.Debugf("serviceDmsgService->readDestUserPubsub: delete msg by key:%v", string(result.Key))
			}

			d.unSubscribeDestUser(pubkey)
			return
		}

		m, err := pubsub.UserSub.Next(d.BaseService.GetCtx())
		if err != nil {
			dmsgLog.Logger.Error(err)
			return
		}
		if d.BaseService.GetHost().ID() == m.ReceivedFrom {
			continue
		}

		dmsgLog.Logger.Debugf("serviceDmsgService->readDestUserPubsub: receive pubsub msg, topic:%s, receivedFrom:%v", *m.Topic, m.ReceivedFrom)

		protocolID, protocolIDLen, err := d.CheckPubsubData(m.Data)
		if err != nil {
			dmsgLog.Logger.Error(err)
			continue
		}
		contentData := m.Data[protocolIDLen:]
		reqSubscribe := d.PubsubProtocolReqSubscribes[protocolID]
		if reqSubscribe != nil {
			reqSubscribe.OnRequest(m, contentData)
			continue
		} else {
			dmsgLog.Logger.Warnf("serviceDmsgService->readDestUserPubsub: no find protocolId(%d) for reqSubscribe", protocolID)
		}
		// no protocol for resSubScribe in service
		resSubScribe := d.PubsubProtocolResSubscribes[protocolID]
		if resSubScribe != nil {
			resSubScribe.OnResponse(m, contentData)
			continue
		} else {
			dmsgLog.Logger.Warnf("serviceDmsgService->readDestUserPubsub: no find protocolId(%d) for resSubscribe", protocolID)
		}
	}
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
	go d.BaseService.DiscoverRendezvousPeers()

	d.destUserPubsubs[userPubKey] = &dmsgServiceCommon.DestUserPubsub{
		UserTopic:           userTopic,
		UserSub:             userSub,
		MsgRWMutex:          sync.RWMutex{},
		LastReciveTimestamp: time.Now().Unix(),
	}
	return nil
}

func (d *DmsgService) unSubscribeDestUser(userPubKey string) error {
	userPubsub := d.destUserPubsubs[userPubKey]
	if userPubsub == nil {
		dmsgLog.Logger.Errorf("serviceDmsgService->unSubscribeDestUser: no find userPubkey %s for pubsub", userPubKey)
		return fmt.Errorf("serviceDmsgService->unSubscribeDestUser: no find userPubkey %s for pubsub", userPubKey)
	}
	userPubsub.UserSub.Cancel()
	err := userPubsub.UserTopic.Close()
	if err != nil {
		dmsgLog.Logger.Warnln(err)
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

func (d *DmsgService) getDestUserPubsub(userPubkey string) *dmsgServiceCommon.DestUserPubsub {
	return d.destUserPubsubs[userPubkey]
}

func (d *DmsgService) saveUserMsg(protoMsg protoreflect.ProtoMessage, msgContent []byte) error {
	sendMsgReq, ok := protoMsg.(*pb.SendMsgReq)
	if !ok {
		dmsgLog.Logger.Errorf("serviceDmsgService->saveUserMsg: cannot convert %v to *pb.SendMsgReq", protoMsg)
		return fmt.Errorf("serviceDmsgService->saveUserMsg: cannot convert %v to *pb.SendMsgReq", protoMsg)
	}

	pubkey := sendMsgReq.BasicData.DestPubkey
	pubsub := d.getDestUserPubsub(pubkey)
	if pubsub == nil {
		dmsgLog.Logger.Errorf("serviceDmsgService->saveUserMsg: cannot find src user pubsub for %v", pubkey)
		return fmt.Errorf("serviceDmsgService->saveUserMsg: cannot find src user pubsub for %v", pubkey)
	}

	pubsub.MsgRWMutex.RLock()
	defer pubsub.MsgRWMutex.RUnlock()

	key := d.GetFullFromMsgPrefix(sendMsgReq)
	err := d.Datastore.Put(d.BaseService.GetCtx(), ds.NewKey(key), sendMsgReq.MsgContent)
	if err != nil {
		return err
	}

	return err
}

func (d *DmsgService) isAvailableMailbox(userPubKey string) bool {
	destUserCount := len(d.destUserPubsubs)
	cfg := d.BaseService.GetConfig()
	if destUserCount >= cfg.DMsg.MaxMailboxPubsubCount {
		dmsgLog.Logger.Warnf("serviceDmsgService->isAvailableMailbox: exceeded the maximum number of mailbox services, current destUserCount:%v", destUserCount)
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
	return nil
}

func (d *DmsgService) Stop() error {
	err := d.DmsgService.Stop()
	if err != nil {
		return err
	}

	d.unSubscribeDestUsers()
	return nil
}

func (d *DmsgService) GetDestUserPubsub(publicKey string) *dmsgServiceCommon.DestUserPubsub {
	return d.destUserPubsubs[publicKey]
}

// StreamProtocolCallback interface
func (d *DmsgService) OnCreateMailboxRequest(protoData protoreflect.ProtoMessage) (interface{}, error) {
	request, ok := protoData.(*pb.CreateMailboxReq)
	if !ok {
		dmsgLog.Logger.Errorf("serviceDmsgService->OnCreateMailboxRequest: cannot convert %v to *pb.CreateMailboxReq", protoData)
		return nil, fmt.Errorf("serviceDmsgService->OnCreateMailboxRequest: cannot convert %v to *pb.CreateMailboxReq", protoData)
	}
	isAvailable := d.isAvailableMailbox(request.BasicData.DestPubkey)
	if !isAvailable {
		return nil, errors.New(dmsgServiceCommon.MailboxLimitErr)
	}
	err := d.publishDestUser(request.BasicData.DestPubkey)
	if err != nil {
		dmsgLog.Logger.Warnf("serviceDmsgService->OnCreateMailboxResponse: destPubkey %s already exist", request.BasicData.DestPubkey)
	}
	return nil, nil
}

func (d *DmsgService) OnCreateMailboxResponse(protoData protoreflect.ProtoMessage) (interface{}, error) {
	response, ok := protoData.(*pb.CreateMailboxRes)
	if !ok {
		dmsgLog.Logger.Errorf("serviceDmsgService->OnCreateMailboxResponse: cannot convert %v to *pb.CreateMailboxRes", protoData)
		return nil, fmt.Errorf("serviceDmsgService->OnCreateMailboxResponse: cannot convert %v to *pb.CreateMailboxRes", protoData)
	}

	pubsub := d.getDestUserPubsub(response.BasicData.DestPubkey)
	if pubsub != nil {
		dmsgLog.Logger.Warnf("serviceDmsgService->OnCreateMailboxResponse: destPubkey %s already exist", response.BasicData.DestPubkey)
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
		dmsgLog.Logger.Errorf("serviceDmsgService->OnReleaseMailboxRequest: cannot convert %v to *pb.ReleaseMailboxReq", protoData)
		return nil, fmt.Errorf("serviceDmsgService->OnReleaseMailboxRequest: cannot convert %v to *pb.ReleaseMailboxReq", protoData)
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
		dmsgLog.Logger.Errorf("serviceDmsgService->OnReadMailboxMsgRequest: cannot convert %v to *pb.ReadMailboxMsgReq", protoData)
		return nil, fmt.Errorf("serviceDmsgService->OnReadMailboxMsgRequest: cannot convert %v to *pb.ReadMailboxMsgReq", protoData)
	}

	pubkey := request.BasicData.DestPubkey
	pubsub := d.getDestUserPubsub(pubkey)
	if pubsub == nil {
		dmsgLog.Logger.Errorf("serviceDmsgService->OnReadMailboxMsgRequest: cannot find src user pubsub for %s", pubkey)
		return nil, fmt.Errorf("serviceDmsgService->OnReadMailboxMsgRequest: cannot find src user pubsub for %s", pubkey)
	}
	pubsub.MsgRWMutex.RLock()
	defer pubsub.MsgRWMutex.RUnlock()

	var query = query.Query{
		Prefix: d.getMsgPrefix(pubkey),
	}
	results, err := d.Datastore.Query(d.BaseService.GetCtx(), query)
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
		dmsgLog.Logger.Debugf("serviceDmsgService->OnReadMailboxMsgRequest key:%v, value:%v", string(result.Key), string(result.Value))
		mailboxMsgDataList = append(mailboxMsgDataList, mailboxMsgData)
		needDeleteKeyList = append(needDeleteKeyList, string(result.Key))
		find = true
	}

	for _, needDeleteKey := range needDeleteKeyList {
		err := d.Datastore.Delete(d.BaseService.GetCtx(), ds.NewKey(needDeleteKey))
		if err != nil {
			dmsgLog.Logger.Errorf("serviceDmsgService->OnReadMailboxMsgRequest: delete msg happen err: %v", err)
		}
	}

	if !find {
		dmsgLog.Logger.Debug("serviceDmsgService->OnReadMailboxMsgRequest: user msgs is empty")
	}
	return mailboxMsgDataList, nil
}

func (d *DmsgService) OnCustomProtocolRequest(protoData protoreflect.ProtoMessage) (interface{}, error) {
	request, ok := protoData.(*pb.CustomProtocolReq)
	if !ok {
		dmsgLog.Logger.Errorf("serviceDmsgService->OnCustomProtocolRequest: cannot convert %v to *pb.CustomContentReq", protoData)
		return nil, fmt.Errorf("serviceDmsgService->OnCustomProtocolRequest: cannot convert %v to *pb.CustomContentReq", protoData)
	}

	customProtocolInfo := d.customStreamProtocolInfoList[request.CustomProtocolID]
	if customProtocolInfo == nil {
		dmsgLog.Logger.Errorf("serviceDmsgService->OnCustomProtocolRequest: customProtocolInfo is nil, request: %v", request)
		return nil, fmt.Errorf("serviceDmsgService->OnCustomProtocolRequest: customProtocolInfo is nil, request: %v", request)
	}

	if customProtocolInfo.Service == nil {
		dmsgLog.Logger.Errorf("serviceDmsgService->OnCustomProtocolRequest: callback is nil")
		return nil, fmt.Errorf("serviceDmsgService->OnCustomProtocolRequest: callback is nil")
	}
	customProtocolInfo.Service.HandleRequest(request)
	return request, nil
}

func (d *DmsgService) OnCustomProtocolResponse(reqProtoData protoreflect.ProtoMessage, resProtoData protoreflect.ProtoMessage) (interface{}, error) {
	response, ok := resProtoData.(*pb.CustomProtocolRes)
	if !ok {
		dmsgLog.Logger.Errorf("serviceDmsgService->OnCustomProtocolResponse: cannot convert %v to *pb.CustomContentRes", resProtoData)
		return nil, fmt.Errorf("serviceDmsgService->OnCustomProtocolResponse: cannot convert %v to *pb.CustomContentRes", resProtoData)
	}
	customProtocolInfo := d.customStreamProtocolInfoList[response.CustomProtocolID]
	if customProtocolInfo == nil {
		dmsgLog.Logger.Errorf("serviceDmsgService->OnCustomProtocolResponse: customProtocolInfo is nil, response: %v", response)
		return nil, fmt.Errorf("serviceDmsgService->OnCustomProtocolResponse: customProtocolInfo is nil, response: %v", response)
	}
	if customProtocolInfo.Service == nil {
		dmsgLog.Logger.Errorf("serviceDmsgService->OnCustomProtocolResponse: callback is nil")
		return nil, fmt.Errorf("serviceDmsgService->OnCustomProtocolResponse: callback is nil")
	}

	request, ok := reqProtoData.(*pb.CustomProtocolReq)
	if !ok {
		dmsgLog.Logger.Errorf("serviceDmsgService->OnCustomProtocolResponse: cannot convert %v to *pb.CustomContentRes", reqProtoData)
		return nil, fmt.Errorf("serviceDmsgService->OnCustomProtocolResponse: cannot convert %v to *pb.CustomContentRes", reqProtoData)
	}

	err := customProtocolInfo.Service.HandleResponse(request, response)
	if err != nil {
		dmsgLog.Logger.Errorf("serviceDmsgService->OnCustomProtocolResponse: callback happen err: %v", err)
		return nil, err
	}
	return nil, nil
}

// PubsubProtocolCallback interface
func (d *DmsgService) OnSeekMailboxRequest(protoData protoreflect.ProtoMessage) (interface{}, error) {
	request, ok := protoData.(*pb.SeekMailboxReq)
	if !ok {
		tvLog.Logger.Errorf("serviceDmsgService->OnSeekMailboxRequest: cannot convert %v to *pb.SeekMailboxReq", protoData)
		return nil, fmt.Errorf("serviceDmsgService->OnSeekMailboxRequest: cannot convert %v to *pb.SeekMailboxReq", protoData)
	}
	pubsub := d.destUserPubsubs[request.BasicData.DestPubkey]
	if pubsub == nil {
		tvLog.Logger.Errorf("serviceDmsgService->OnSeekMailboxRequest: mailbox not exist for pubic key:%v", request.BasicData.DestPubkey)
		return nil, fmt.Errorf("serviceDmsgService->OnSeekMailboxRequest: mailbox not exist for pubic key:%v", request.BasicData.DestPubkey)
	}
	return nil, nil
}

func (d *DmsgService) OnHandleSendMsgRequest(protoMsg protoreflect.ProtoMessage, protoData []byte) (interface{}, error) {
	request, ok := protoMsg.(*pb.SendMsgReq)
	if !ok {
		dmsgLog.Logger.Errorf("serviceDmsgService->OnHandleSendMsgRequest: cannot convert %v to *pb.SendMsgReq", protoMsg)
		return nil, fmt.Errorf("serviceDmsgService->OnHandleSendMsgRequest: cannot convert %v to *pb.SendMsgReq", protoMsg)
	}
	pubkey := request.BasicData.DestPubkey
	pubsub := d.getDestUserPubsub(pubkey)
	if pubsub == nil {
		dmsgLog.Logger.Errorf("serviceDmsgService->OnHandleSendMsgRequest: public key %s is not exist", pubkey)
		return nil, fmt.Errorf("serviceDmsgService->OnHandleSendMsgRequest: public key %s is not exist", pubkey)
	}
	pubsub.LastReciveTimestamp = time.Now().Unix()
	d.saveUserMsg(protoMsg, protoData)
	return nil, nil
}

func (d *DmsgService) PublishProtocol(protocolID pb.ProtocolID, userPubkey string, protocolData []byte) error {
	userPubsub := d.getDestUserPubsub(userPubkey)
	if userPubsub == nil {
		dmsgLog.Logger.Errorf("serviceDmsgService->PublishProtocol: no find user pubic key %s for pubsub", userPubkey)
		return fmt.Errorf("serviceDmsgService->PublishProtocol: no find user pubic key %s for pubsub", userPubkey)
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
	if err := userPubsub.UserTopic.Publish(d.BaseService.GetCtx(), pubsubData); err != nil {
		dmsgLog.Logger.Errorf("serviceDmsgService->PublishProtocol: publish protocol err: %v", err)
		return fmt.Errorf("serviceDmsgService->PublishProtocol: publish protocol err: %v", err)
	}
	return nil
}

// custom protocol
func (d *DmsgService) RegistCustomStreamProtocol(service customProtocol.CustomStreamProtocolService) error {
	customProtocolID := service.GetProtocolID()
	if d.customStreamProtocolInfoList[customProtocolID] != nil {
		dmsgLog.Logger.Errorf("serviceDmsgService->RegistCustomStreamProtocol: protocol %s is already exist", customProtocolID)
		return fmt.Errorf("serviceDmsgService->RegistCustomStreamProtocol: protocol %s is already exist", customProtocolID)
	}
	d.customStreamProtocolInfoList[customProtocolID] = &dmsgServiceCommon.CustomProtocolInfo{
		StreamProtocol: serviceProtocol.NewCustomStreamProtocol(d.BaseService.GetCtx(), d.BaseService.GetHost(), customProtocolID, d),
		Service:        service,
	}
	service.SetCtx(d.BaseService.GetCtx())
	return nil
}

func (d *DmsgService) UnregistCustomStreamProtocol(callback customProtocol.CustomStreamProtocolService) error {
	customProtocolID := callback.GetProtocolID()
	if d.customStreamProtocolInfoList[customProtocolID] == nil {
		dmsgLog.Logger.Warnf("serviceDmsgService->UnregistCustomStreamProtocol: protocol %s is not exist", customProtocolID)
		return nil
	}
	d.customStreamProtocolInfoList[customProtocolID] = &dmsgServiceCommon.CustomProtocolInfo{
		StreamProtocol: serviceProtocol.NewCustomStreamProtocol(d.BaseService.GetCtx(), d.BaseService.GetHost(), customProtocolID, d),
		Service:        callback,
	}
	return nil
}

func daysBetween(start, end int64) int {
	startTime := time.Unix(start, 0)
	endTime := time.Unix(end, 0)
	return int(endTime.Sub(startTime).Hours() / 24)
}
