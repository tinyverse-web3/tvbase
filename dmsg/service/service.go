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
	tvLog "github.com/tinyverse-web3/tvbase/common/log"
	"github.com/tinyverse-web3/tvbase/dmsg"
	dmsgLog "github.com/tinyverse-web3/tvbase/dmsg/common/log"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol"
	customProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol/custom"
	"github.com/tinyverse-web3/tvbase/dmsg/service/common"
	serviceProtocol "github.com/tinyverse-web3/tvbase/dmsg/service/protocol"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type ProtocolProxy struct {
	createMailboxProtocol *common.StreamProtocol
	releaseMailboxPrtocol *common.StreamProtocol
	readMailboxMsgPrtocol *common.StreamProtocol
	seekMailboxProtocol   *common.PubsubProtocol
	sendMsgPrtocol        *serviceProtocol.SendMsgProtocol
}

type DmsgService struct {
	dmsg.DmsgService
	ProtocolProxy
	DestUserPubsubs map[string]*common.DestUserPubsub
	// service
	protocolReqSubscribes  map[pb.ProtocolID]protocol.ReqSubscribe
	protocolResSubscribes  map[pb.ProtocolID]protocol.ResSubscribe
	customProtocolInfoList map[string]*common.CustomProtocolInfo
}

func CreateService(nodeService tvCommon.NodeService) (*DmsgService, error) {
	d := &DmsgService{}
	err := d.Init(nodeService)
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (d *DmsgService) Init(nodeService tvCommon.NodeService) error {
	err := d.DmsgService.Init(nodeService)
	if err != nil {
		return err
	}

	// stream protocol
	d.createMailboxProtocol = serviceProtocol.NewCreateMailboxProtocol(d.NodeService.GetCtx(), d.NodeService.GetHost(), d)
	d.releaseMailboxPrtocol = serviceProtocol.NewReleaseMailboxProtocol(d.NodeService.GetCtx(), d.NodeService.GetHost(), d)
	d.readMailboxMsgPrtocol = serviceProtocol.NewReadMailboxMsgProtocol(d.NodeService.GetCtx(), d.NodeService.GetHost(), d)

	// pubsub protocol
	d.seekMailboxProtocol = serviceProtocol.NewSeekMailboxProtocol(d.NodeService.GetHost(), d, d)
	d.sendMsgPrtocol = serviceProtocol.NewSendMsgProtocol(d.NodeService.GetHost(), d, d)

	d.DestUserPubsubs = make(map[string]*common.DestUserPubsub)

	d.protocolReqSubscribes = make(map[pb.ProtocolID]protocol.ReqSubscribe)
	d.protocolResSubscribes = make(map[pb.ProtocolID]protocol.ResSubscribe)

	d.customProtocolInfoList = make(map[string]*common.CustomProtocolInfo)
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

func (d *DmsgService) readDestUserPubsub(pubkey string, pubsub *common.DestUserPubsub) {
	for {
		days := daysBetween(pubsub.LastReciveTimestamp, time.Now().Unix())
		// delete mailbox msg in datastore and cancel mailbox subscribe when days is over, default day is 30
		cfg := d.NodeService.GetConfig()
		if days >= cfg.DMsg.KeepMailboxMsgDay {
			var query = query.Query{
				Prefix:   d.getMsgPrefix(pubkey),
				KeysOnly: true,
			}
			results, err := d.Datastore.Query(d.NodeService.GetCtx(), query)
			if err != nil {
				dmsgLog.Logger.Errorf("serviceDmsgService->readDestUserPubsub: query error: %v", err)
			}

			for result := range results.Next() {
				d.Datastore.Delete(d.NodeService.GetCtx(), ds.NewKey(result.Key))
				dmsgLog.Logger.Debugf("serviceDmsgService->readDestUserPubsub: delete msg by key:%v", string(result.Key))
			}

			d.unSubscribeDestUser(pubkey)
			return
		}

		m, err := pubsub.UserSub.Next(d.NodeService.GetCtx())
		if err != nil {
			dmsgLog.Logger.Error(err)
			return
		}
		if d.NodeService.GetHost().ID() == m.ReceivedFrom {
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
	go d.NodeService.DiscoverRendezvousPeers()

	d.DestUserPubsubs[userPubKey] = &common.DestUserPubsub{
		UserTopic:           userTopic,
		UserSub:             userSub,
		MsgRWMutex:          sync.RWMutex{},
		LastReciveTimestamp: time.Now().Unix(),
	}
	return nil
}

func (d *DmsgService) unSubscribeDestUser(userPubKey string) error {
	userPubsub := d.DestUserPubsubs[userPubKey]
	if userPubsub == nil {
		dmsgLog.Logger.Errorf("serviceDmsgService->unSubscribeDestUser: no find userPubkey %s for pubsub", userPubKey)
		return fmt.Errorf("serviceDmsgService->unSubscribeDestUser: no find userPubkey %s for pubsub", userPubKey)
	}
	userPubsub.UserSub.Cancel()
	err := userPubsub.UserTopic.Close()
	if err != nil {
		dmsgLog.Logger.Warnln(err)
	}
	delete(d.DestUserPubsubs, userPubKey)
	return nil
}

func (d *DmsgService) unSubscribeDestUsers() error {
	for userPubKey := range d.DestUserPubsubs {
		d.unSubscribeDestUser(userPubKey)
	}
	return nil
}

func (d *DmsgService) getDestUserPubsub(userPubkey string) *common.DestUserPubsub {
	return d.DestUserPubsubs[userPubkey]
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
	err := d.Datastore.Put(d.NodeService.GetCtx(), ds.NewKey(key), sendMsgReq.MsgContent)
	if err != nil {
		return err
	}

	return err
}

func (d *DmsgService) isAvailableMailbox(userPubKey string) bool {
	destUserCount := len(d.DestUserPubsubs)
	cfg := d.NodeService.GetConfig()
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

// StreamProtocolCallback interface
func (d *DmsgService) OnCreateMailboxRequest(protoData protoreflect.ProtoMessage) (interface{}, error) {
	request, ok := protoData.(*pb.CreateMailboxReq)
	if !ok {
		dmsgLog.Logger.Errorf("serviceDmsgService->OnCreateMailboxRequest: cannot convert %v to *pb.CreateMailboxReq", protoData)
		return nil, fmt.Errorf("serviceDmsgService->OnCreateMailboxRequest: cannot convert %v to *pb.CreateMailboxReq", protoData)
	}
	isAvailable := d.isAvailableMailbox(request.BasicData.DestPubkey)
	if !isAvailable {
		return nil, errors.New(common.MailboxLimitErr)
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
			Code:   common.MailboxAlreadyExistCode,
			Result: common.MailboxAlreadyExistErr,
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
	results, err := d.Datastore.Query(d.NodeService.GetCtx(), query)
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
		err := d.Datastore.Delete(d.NodeService.GetCtx(), ds.NewKey(needDeleteKey))
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

	customProtocolInfo := d.customProtocolInfoList[request.CustomProtocolID]
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
	customProtocolInfo := d.customProtocolInfoList[response.CustomProtocolID]
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
	pubsub := d.DestUserPubsubs[request.BasicData.DestPubkey]
	if pubsub == nil {
		tvLog.Logger.Errorf("serviceDmsgService->OnSeekMailboxRequest: mailbox not exist for pubic key:%v", request.BasicData.DestPubkey)
		return nil, fmt.Errorf("serviceDmsgService->OnSeekMailboxRequest: mailbox not exist for pubic key:%v", request.BasicData.DestPubkey)
	}
	return nil, nil
}

func (d *DmsgService) OnSendMsgResquest(protoMsg protoreflect.ProtoMessage, protoData []byte) (interface{}, error) {
	request, ok := protoMsg.(*pb.SendMsgReq)
	if !ok {
		dmsgLog.Logger.Errorf("serviceDmsgService->OnSendMsgResquest: cannot convert %v to *pb.SendMsgReq", protoMsg)
		return nil, fmt.Errorf("serviceDmsgService->OnSendMsgResquest: cannot convert %v to *pb.SendMsgReq", protoMsg)
	}
	pubkey := request.BasicData.DestPubkey
	pubsub := d.getDestUserPubsub(pubkey)
	if pubsub == nil {
		dmsgLog.Logger.Errorf("serviceDmsgService->OnSendMsgResquest: public key %s is not exist", pubkey)
		return nil, fmt.Errorf("serviceDmsgService->OnSendMsgResquest: public key %s is not exist", pubkey)
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
	if err := userPubsub.UserTopic.Publish(d.NodeService.GetCtx(), pubsubData); err != nil {
		dmsgLog.Logger.Errorf("serviceDmsgService->PublishProtocol: publish protocol err: %v", err)
		return fmt.Errorf("serviceDmsgService->PublishProtocol: publish protocol err: %v", err)
	}
	return nil
}

// custom protocol
func (d *DmsgService) RegistCustomStreamProtocol(service customProtocol.CustomProtocolService) error {
	customProtocolID := service.GetProtocolID()
	if d.customProtocolInfoList[customProtocolID] != nil {
		dmsgLog.Logger.Errorf("serviceDmsgService->RegistCustomStreamProtocol: protocol %s is already exist", customProtocolID)
		return fmt.Errorf("serviceDmsgService->RegistCustomStreamProtocol: protocol %s is already exist", customProtocolID)
	}
	d.customProtocolInfoList[customProtocolID] = &common.CustomProtocolInfo{
		StreamProtocol: serviceProtocol.NewCustomProtocol(d.NodeService.GetCtx(), d.NodeService.GetHost(), customProtocolID, d),
		Service:        service,
	}
	service.SetCtx(d.NodeService.GetCtx())
	return nil
}

func (d *DmsgService) UnregistCustomStreamProtocol(callback customProtocol.CustomProtocolService) error {
	customProtocolID := callback.GetProtocolID()
	if d.customProtocolInfoList[customProtocolID] == nil {
		dmsgLog.Logger.Warnf("serviceDmsgService->UnregistCustomStreamProtocol: protocol %s is not exist", customProtocolID)
		return nil
	}
	d.customProtocolInfoList[customProtocolID] = &common.CustomProtocolInfo{
		StreamProtocol: serviceProtocol.NewCustomProtocol(d.NodeService.GetCtx(), d.NodeService.GetHost(), customProtocolID, d),
		Service:        callback,
	}
	return nil
}

func daysBetween(start, end int64) int {
	startTime := time.Unix(start, 0)
	endTime := time.Unix(end, 0)
	return int(endTime.Sub(startTime).Hours() / 24)
}
