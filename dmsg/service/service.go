package service

import (
	"errors"
	"fmt"
	"sync"
	"time"

	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	tvCommon "github.com/tinyverse-web3/tvbase/common"
	tvbaseConfig "github.com/tinyverse-web3/tvbase/common/config"
	"github.com/tinyverse-web3/tvbase/common/db"
	"github.com/tinyverse-web3/tvbase/dmsg"
	dmsgKey "github.com/tinyverse-web3/tvbase/dmsg/common/key"
	dmsgLog "github.com/tinyverse-web3/tvbase/dmsg/common/log"
	"github.com/tinyverse-web3/tvbase/dmsg/common/msg"
	dmsgUser "github.com/tinyverse-web3/tvbase/dmsg/common/user"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
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
	sendMsgProtocol       *serviceProtocol.SendMsgProtocol
}

type DmsgService struct {
	dmsg.DmsgService
	ProtocolProxy
	user                         *dmsgUser.User
	onReceiveMsg                 msg.OnReceiveMsg
	destUserList                 map[string]*dmsgUser.DestUser
	channelList                  map[string]*dmsgUser.Channel
	customStreamProtocolInfoList map[string]*dmsgServiceCommon.CustomStreamProtocolInfo
	customPubsubProtocolInfoList map[string]*dmsgServiceCommon.CustomPubsubProtocolInfo
	datastore                    db.Datastore
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
	if cfg.Mode == tvbaseConfig.ServiceMode {
		d.datastore, err = db.CreateDataStore(cfg.DMsg.DatastorePath, cfg.Mode)
		if err != nil {
			dmsgLog.Logger.Errorf("dmsgService->Init: create datastore error %v", err)
			return err
		}
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

	d.destUserList = make(map[string]*dmsgUser.DestUser)
	d.channelList = make(map[string]*dmsgUser.Channel)

	d.customStreamProtocolInfoList = make(map[string]*dmsgServiceCommon.CustomStreamProtocolInfo)
	d.customPubsubProtocolInfoList = make(map[string]*dmsgServiceCommon.CustomPubsubProtocolInfo)

	d.stopCleanRestResource = make(chan bool)
	return nil
}

// for sdk
func (d *DmsgService) Start() error {
	cfg := d.BaseService.GetConfig()
	if cfg.Mode == tvbaseConfig.ServiceMode {
		d.cleanRestResource()
	}
	return nil
}

func (d *DmsgService) Stop() error {
	d.unsubscribeUser()
	d.UnsubscribeDestUserList()
	d.UnsubscribeChannelList()

	cfg := d.BaseService.GetConfig()
	if cfg.Mode == tvbaseConfig.ServiceMode {
		d.stopCleanRestResource <- true
	}
	return nil
}

func (d *DmsgService) InitUser(pubkeyData []byte, getSig dmsgKey.GetSigCallback, opts ...any) error {
	dmsgLog.Logger.Debug("DmsgService->InitUser begin")
	pubkey := tvutilKey.TranslateKeyProtoBufToString(pubkeyData)
	err := d.subscribeUser(pubkey, getSig)
	if err != nil {
		return err
	}

	cfg := d.BaseService.GetConfig()
	if cfg.Mode == tvbaseConfig.ServiceMode {
		return nil
	}
	defTimeout := 24 * time.Hour
	if len(opts) > 0 {
		timeout, ok := opts[0].(time.Duration)
		if ok {
			defTimeout = timeout
		}
	}
	select {
	case <-d.BaseService.GetRendezvousChan():
		d.initMailbox(pubkey)
	case <-time.After(defTimeout):
		return nil
	case <-d.BaseService.GetCtx().Done():
		return d.BaseService.GetCtx().Err()
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

func (d *DmsgService) saveUserMsg(protoMsg protoreflect.ProtoMessage) error {
	sendMsgReq, ok := protoMsg.(*pb.SendMsgReq)
	if !ok {
		dmsgLog.Logger.Errorf("dmsgService->saveUserMsg: cannot convert %v to *pb.SendMsgReq", protoMsg)
		return fmt.Errorf("dmsgService->saveUserMsg: cannot convert %v to *pb.SendMsgReq", protoMsg)
	}

	pubkey := sendMsgReq.BasicData.Pubkey
	user := d.GetDestUser(pubkey)
	if user == nil {
		dmsgLog.Logger.Errorf("dmsgService->saveUserMsg: cannot find src user pubsub for %v", pubkey)
		return fmt.Errorf("dmsgService->saveUserMsg: cannot find src user pubsub for %v", pubkey)
	}

	user.MsgRWMutex.RLock()
	defer user.MsgRWMutex.RUnlock()

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

// destuser
func (d *DmsgService) SubscribeDestUser(pubkey string) error {
	dmsgLog.Logger.Debug("DmsgService->SubscribeDestUser begin\npubkey: %s", pubkey)
	if d.destUserList[pubkey] != nil {
		dmsgLog.Logger.Errorf("DmsgService->SubscribeDestUser: pubkey is already exist in destUserList")
		return fmt.Errorf("DmsgService->SubscribeDestUser: pubkey is already exist in destUserList")
	}
	target, err := dmsgUser.NewTarget(d.BaseService.GetCtx(), d.Pubsub, pubkey, nil)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->SubscribeDestUser: NewTarget error: %v", err)
		return err
	}
	user := &dmsgUser.DestUser{
		DestTarget: dmsgUser.DestTarget{
			Target:              *target,
			LastReciveTimestamp: time.Now().UnixNano(),
		},
		MsgRWMutex: sync.RWMutex{},
	}

	d.destUserList[pubkey] = user

	go d.HandleProtocolWithPubsub(&user.Target)
	return nil
}

func (d *DmsgService) UnsubscribeDestUser(pubkey string) error {
	dmsgLog.Logger.Debugf("DmsgService->UnSubscribeDestUser begin\npubkey: %s", pubkey)

	user := d.destUserList[pubkey]
	if user == nil {
		dmsgLog.Logger.Errorf("DmsgService->UnSubscribeDestUser: pubkey is not exist in destUserList")
		return fmt.Errorf("DmsgService->UnSubscribeDestUser: pubkey is not exist in destUserList")
	}
	user.Close()
	delete(d.destUserList, pubkey)

	dmsgLog.Logger.Debug("DmsgService->unSubscribeDestUser end")
	return nil
}

func (d *DmsgService) UnsubscribeDestUserList() error {
	for userPubKey := range d.destUserList {
		d.UnsubscribeDestUser(userPubKey)
	}
	return nil
}

func (d *DmsgService) GetDestUser(pubkey string) *dmsgUser.DestUser {
	return d.destUserList[pubkey]
}

// channel
func (d *DmsgService) isAvailablePubChannel(pubKey string) bool {
	pubChannelInfo := len(d.channelList)
	cfg := d.BaseService.GetConfig()
	if pubChannelInfo >= cfg.DMsg.MaxPubChannelPubsubCount {
		dmsgLog.Logger.Warnf("dmsgService->isAvailableMailbox: exceeded the maximum number of mailbox services, current destUserCount:%v", pubChannelInfo)
		return false
	}
	return true
}

func (d *DmsgService) SubscribeChannel(pubkey string) error {
	dmsgLog.Logger.Debugf("DmsgService->SubscribeChannel begin:\npubkey: %s", pubkey)

	if d.channelList[pubkey] != nil {
		dmsgLog.Logger.Errorf("DmsgService->SubscribeChannel: pubkey is already exist in channelList")
		return fmt.Errorf("DmsgService->SubscribeChannel: pubkey is already exist in channelList")
	}

	target, err := dmsgUser.NewTarget(d.BaseService.GetCtx(), d.Pubsub, pubkey, nil)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->SubscribeChannel: NewTarget error: %v", err)
		return err
	}

	channel := &dmsgUser.Channel{
		DestTarget: dmsgUser.DestTarget{
			Target:              *target,
			LastReciveTimestamp: time.Now().UnixNano(),
		},
	}
	d.channelList[pubkey] = channel

	// go d.BaseService.DiscoverRendezvousPeers()
	go d.HandleProtocolWithPubsub(&channel.Target)

	dmsgLog.Logger.Debug("DmsgService->SubscribeChannel end")
	return nil
}

func (d *DmsgService) UnsubscribeChannel(pubkey string) error {
	dmsgLog.Logger.Debugf("DmsgService->UnsubscribeChannel begin\npubKey: %s", pubkey)

	channel := d.channelList[pubkey]
	if channel == nil {
		dmsgLog.Logger.Errorf("DmsgService->UnsubscribeChannel: channel is nil")
		return fmt.Errorf("DmsgService->UnsubscribeChannel: channel is nil")
	}
	err := channel.Close()
	if err != nil {
		dmsgLog.Logger.Warnf("DmsgService->UnsubscribeChannel: channel.Close error: %v", err)
	}
	delete(d.channelList, pubkey)

	dmsgLog.Logger.Debug("DmsgService->UnsubscribeChannel end")
	return nil
}

func (d *DmsgService) UnsubscribeChannelList() error {
	for pubkey := range d.channelList {
		d.UnsubscribeChannel(pubkey)
	}
	return nil
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
	user := d.GetDestUser(request.BasicData.Pubkey)
	if user != nil {
		dmsgLog.Logger.Errorf("dmsgService->OnCreateMailboxRequest: user public key pubsub already exist")
		retCode := &pb.RetCode{
			Code:   dmsgServiceCommon.MailboxAlreadyExistCode,
			Result: "dmsgService->OnCreateMailboxRequest: user public key pubsub already exist",
		}
		return nil, retCode, nil
	}

	err := d.SubscribeDestUser(request.BasicData.Pubkey)
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

	err := d.UnsubscribeDestUser(request.BasicData.Pubkey)
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
	pubsub := d.GetDestUser(pubkey)
	if pubsub == nil {
		dmsgLog.Logger.Errorf("dmsgService->OnReadMailboxMsgRequest: cannot find src user pubsub for %s", pubkey)
		return nil, nil, fmt.Errorf("dmsgService->OnReadMailboxMsgRequest: cannot find src user pubsub for %s", pubkey)
	}
	pubsub.MsgRWMutex.RLock()
	defer pubsub.MsgRWMutex.RUnlock()

	var query = query.Query{
		Prefix: d.GetMsgPrefix(pubkey),
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
	pubChannelInfo := d.channelList[channelKey]
	if pubChannelInfo != nil {
		dmsgLog.Logger.Errorf("dmsgService->OnCreateChannelRequest: pubChannelInfo already exist")
		retCode := &pb.RetCode{
			Code:   dmsgServiceCommon.PubChannelAlreadyExistCode,
			Result: "dmsgService->OnCreateChannelRequest: pubChannelInfo already exist",
		}
		return nil, retCode, nil
	}

	err := d.SubscribeChannel(channelKey)
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
	user := d.GetDestUser(pubkey)
	if user == nil {
		dmsgLog.Logger.Errorf("dmsgService->OnSendMsgRequest: public key %s is not exist", pubkey)
		return nil, fmt.Errorf("dmsgService->OnSendMsgRequest: public key %s is not exist", pubkey)
	}
	user.LastReciveTimestamp = time.Now().UnixNano()
	d.saveUserMsg(protoMsg)
	return nil, nil
}

func (d *DmsgService) PublishProtocol(pubkey string, pid pb.PID, protoData []byte) error {
	var target *dmsgUser.Target = nil
	user := d.destUserList[pubkey]
	if user == nil {
		if d.user.Key.PubkeyHex != pubkey {
			channel := d.channelList[pubkey]
			if channel == nil {
				dmsgLog.Logger.Errorf("DmsgService->PublishProtocol: cannot find user/dest/channel for key %s", pubkey)
				return fmt.Errorf("DmsgService->PublishProtocol: cannot find user/dest/channel for key %s", pubkey)
			}
			target = &channel.Target
		} else {
			target = &d.user.Target
		}
	} else {
		target = &user.Target
	}

	buf, err := dmsgProtocol.GenProtoData(pid, protoData)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->PublishProtocol: GenProtoData error: %v", err)
		return err
	}

	if err := target.Publish(buf); err != nil {
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

	user, err := dmsgUser.NewTarget(d.BaseService.GetCtx(), d.Pubsub, pubkey, getSig)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->subscribeUser: NewUser error: %v", err)
		return err
	}

	d.user = &dmsgUser.User{
		Target:        *user,
		ServicePeerID: "",
	}

	// go d.BaseService.DiscoverRendezvousPeers()
	go d.HandleProtocolWithPubsub(&d.user.Target)

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

// channel
func (d *DmsgService) createChannelService(pubkey string) error {
	dmsgLog.Logger.Debugf("DmsgService->createChannelService begin:\n channel key: %s", pubkey)
	find := false

	hostId := d.BaseService.GetHost().ID().String()
	servicePeerList, _ := d.BaseService.GetAvailableServicePeerList(hostId)
	srcPubkey, err := d.GetUserPubkeyHex()
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->createChannelService: GetUserPubkeyHex error: %v", err)
		return err
	}
	peerID := d.BaseService.GetHost().ID().String()
	for _, servicePeerID := range servicePeerList {
		dmsgLog.Logger.Debugf("DmsgService->createChannelService: servicePeerID: %s", servicePeerID)
		if peerID == servicePeerID.String() {
			continue
		}
		_, createChannelDoneChan, err := d.createChannelProtocol.Request(servicePeerID, srcPubkey, pubkey)
		if err != nil {
			continue
		}

		select {
		case createChannelResponseProtoData := <-createChannelDoneChan:
			dmsgLog.Logger.Debugf("DmsgService->createChannelService:\ncreateChannelResponseProtoData: %+v",
				createChannelResponseProtoData)
			response, ok := createChannelResponseProtoData.(*pb.CreateChannelRes)
			if !ok || response == nil {
				dmsgLog.Logger.Errorf("DmsgService->createChannelService: createPubChannelResponseProtoData is not CreatePubChannelRes")
				continue
			}
			if response.RetCode.Code < 0 {
				dmsgLog.Logger.Errorf("DmsgService->createChannelService: createPubChannel fail")
				continue
			} else {
				dmsgLog.Logger.Debugf("DmsgService->createChannelService: createPubChannel success")
				find = true
				return nil
			}

		case <-time.After(time.Second * 3):
			continue
		case <-d.BaseService.GetCtx().Done():
			return fmt.Errorf("DmsgService->createChannelService: BaseService.GetCtx().Done()")
		}
	}
	if !find {
		dmsgLog.Logger.Error("DmsgService->createChannelService: no available service peer")
		return fmt.Errorf("DmsgService->createChannelService: no available service peer")
	}
	dmsgLog.Logger.Debug("DmsgService->createChannelService end")
	return nil
}

// common
func (d *DmsgService) cleanRestResource() {
	go func() {
		ticker := time.NewTicker(3 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-d.stopCleanRestResource:
				return
			case <-ticker.C:
				cfg := d.BaseService.GetConfig()
				for pubkey, pubsub := range d.destUserList {
					days := daysBetween(pubsub.LastReciveTimestamp, time.Now().UnixNano())
					// delete mailbox msg in datastore and cancel mailbox subscribe when days is over, default days is 30
					if days >= cfg.DMsg.KeepMailboxMsgDay {
						var query = query.Query{
							Prefix:   d.GetMsgPrefix(pubkey),
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

						d.UnsubscribeDestUser(pubkey)
						return
					}
				}
				for pubChannelPubkey, pubsub := range d.channelList {
					days := daysBetween(pubsub.LastReciveTimestamp, time.Now().UnixNano())
					// delete mailbox msg in datastore and cancel mailbox subscribe when days is over, default days is 30
					if days >= cfg.DMsg.KeepPubChannelDay {
						d.UnsubscribeChannel(pubChannelPubkey)
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

func daysBetween(start, end int64) int {
	startTime := time.Unix(start, 0)
	endTime := time.Unix(end, 0)
	return int(endTime.Sub(startTime).Hours() / 24)
}
