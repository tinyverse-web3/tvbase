package service

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	"github.com/libp2p/go-libp2p/core/peer"
	tvCommon "github.com/tinyverse-web3/tvbase/common"
	"github.com/tinyverse-web3/tvbase/common/db"
	"github.com/tinyverse-web3/tvbase/common/define"
	"github.com/tinyverse-web3/tvbase/dmsg"
	dmsgKey "github.com/tinyverse-web3/tvbase/dmsg/common/key"
	dmsgLog "github.com/tinyverse-web3/tvbase/dmsg/common/log"
	"github.com/tinyverse-web3/tvbase/dmsg/common/msg"
	dmsgUser "github.com/tinyverse-web3/tvbase/dmsg/common/user"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/adapter"
	customProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol/custom"
	tvutilKey "github.com/tinyverse-web3/tvutil/key"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type ProtocolProxy struct {
	createMailboxProtocol *dmsgProtocol.StreamProtocol
	releaseMailboxPrtocol *dmsgProtocol.StreamProtocol
	readMailboxMsgPrtocol *dmsgProtocol.StreamProtocol
	createChannelProtocol *dmsgProtocol.StreamProtocol
	seekMailboxProtocol   *dmsgProtocol.PubsubProtocol
	sendMsgProtocol       *dmsgProtocol.PubsubProtocol
}

type DmsgService struct {
	dmsg.DmsgService
	ProtocolProxy
	user                                *dmsgUser.User
	onReceiveMsg                        msg.OnReceiveMsg
	destUserList                        map[string]*dmsgUser.DestUser
	channelList                         map[string]*dmsgUser.Channel
	customStreamProtocolServiceInfoList map[string]*customProtocol.CustomStreamProtocolServiceInfo
	customStreamProtocolClientInfoList  map[string]*customProtocol.CustomStreamProtocolClientInfo
	datastore                           db.Datastore
	stopCleanRestResource               chan bool
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
	if cfg.Mode == define.ServiceMode {
		d.datastore, err = db.CreateDataStore(cfg.DMsg.DatastorePath, cfg.Mode)
		if err != nil {
			dmsgLog.Logger.Errorf("dmsgService->Init: create datastore error %v", err)
			return err
		}
	}

	// stream protocol
	d.createMailboxProtocol = adapter.NewCreateMailboxProtocol(d.BaseService.GetCtx(), d.BaseService.GetHost(), d, d)
	d.releaseMailboxPrtocol = adapter.NewReleaseMailboxProtocol(d.BaseService.GetCtx(), d.BaseService.GetHost(), d, d)
	d.readMailboxMsgPrtocol = adapter.NewReadMailboxMsgProtocol(d.BaseService.GetCtx(), d.BaseService.GetHost(), d, d)
	d.createChannelProtocol = adapter.NewCreateChannelProtocol(d.BaseService.GetCtx(), d.BaseService.GetHost(), d, d)

	// pubsub protocol
	d.seekMailboxProtocol = adapter.NewSeekMailboxProtocol(d.BaseService.GetCtx(), d.BaseService.GetHost(), d, d)
	d.RegPubsubProtocolReqCallback(d.seekMailboxProtocol.Adapter.GetRequestPID(), d.seekMailboxProtocol)
	d.sendMsgProtocol = adapter.NewSendMsgProtocol(d.BaseService.GetCtx(), d.BaseService.GetHost(), d, d)
	d.RegPubsubProtocolReqCallback(pb.PID_SEND_MSG_REQ, d.sendMsgProtocol)

	d.destUserList = make(map[string]*dmsgUser.DestUser)
	d.channelList = make(map[string]*dmsgUser.Channel)

	d.customStreamProtocolClientInfoList = make(map[string]*customProtocol.CustomStreamProtocolClientInfo)
	d.customStreamProtocolServiceInfoList = make(map[string]*customProtocol.CustomStreamProtocolServiceInfo)

	d.stopCleanRestResource = make(chan bool)
	return nil
}

// sdk-common
func (d *DmsgService) Start() error {
	cfg := d.BaseService.GetConfig()
	if cfg.Mode == define.ServiceMode {
		d.cleanRestResource()
	}
	return nil
}

func (d *DmsgService) Stop() error {
	d.unsubscribeUser()
	d.UnsubscribeDestUserList()
	d.UnsubscribeChannelList()

	cfg := d.BaseService.GetConfig()
	if cfg.Mode == define.ServiceMode {
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
	if cfg.Mode == define.ServiceMode {
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

// sdk-destuser
func (d *DmsgService) GetDestUser(pubkey string) *dmsgUser.DestUser {
	return d.destUserList[pubkey]
}

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
	// go d.BaseService.DiscoverRendezvousPeers()
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

// sdk-channel
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

	cfg := d.BaseService.GetConfig()
	switch cfg.Mode {
	case define.LightMode:
		err = d.createChannelService(channel.Key.PubkeyHex)
		if err != nil {
			target.Close()
			delete(d.channelList, pubkey)
			return err
		}
	}

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

// sdk-msg
func (d *DmsgService) RequestReadMailbox(timeout time.Duration) ([]msg.Msg, error) {
	dmsgLog.Logger.Debugf("DmsgService->RequestReadMailbox begin\ntimeout: %+v", timeout)
	var msgList []msg.Msg
	peerID, err := peer.Decode(d.user.ServicePeerID)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->RequestReadMailbox: peer.Decode error: %v", err)
		return msgList, err
	}
	_, readMailboxDoneChan, err := d.readMailboxMsgPrtocol.Request(peerID, d.user.Key.PubkeyHex)
	if err != nil {
		return msgList, err
	}

	select {
	case responseProtoData := <-readMailboxDoneChan:
		dmsgLog.Logger.Debugf("DmsgService->RequestReadMailbox: responseProtoData: %+v", responseProtoData)
		response, ok := responseProtoData.(*pb.ReadMailboxRes)
		if !ok || response == nil {
			dmsgLog.Logger.Errorf("DmsgService->RequestReadMailbox: readMailboxDoneChan is not ReadMailboxRes")
			return msgList, fmt.Errorf("DmsgService->RequestReadMailbox: readMailboxDoneChan is not ReadMailboxRes")
		}
		dmsgLog.Logger.Debugf("DmsgService->RequestReadMailbox: readMailboxChanDoneChan success")
		msgList, err = d.parseReadMailboxResponse(response, msg.MsgDirection.From)
		if err != nil {
			return msgList, err
		}

		return msgList, nil
	case <-time.After(timeout):
		dmsgLog.Logger.Debugf("DmsgService->RequestReadMailbox: timeout")
		return msgList, fmt.Errorf("DmsgService->RequestReadMailbox: timeout")
	case <-d.BaseService.GetCtx().Done():
		dmsgLog.Logger.Debugf("DmsgService->RequestReadMailbox: BaseService.GetCtx().Done()")
		return msgList, fmt.Errorf("DmsgService->RequestReadMailbox: BaseService.GetCtx().Done()")
	}
}

func (d *DmsgService) SendMsg(destPubkey string, content []byte) (*pb.SendMsgReq, error) {
	dmsgLog.Logger.Debugf("DmsgService->SendMsg begin:\ndestPubkey: %s", destPubkey)
	sigPubkey := d.user.Key.PubkeyHex
	requestProtoData, _, err := d.sendMsgProtocol.Request(
		sigPubkey,
		destPubkey,
		content,
	)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->SendMsg: %v", err)
		return nil, err
	}
	request, ok := requestProtoData.(*pb.SendMsgReq)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->SendMsg: requestProtoData is not SendMsgReq")
		return nil, fmt.Errorf("DmsgService->SendMsg: requestProtoData is not SendMsgReq")
	}
	dmsgLog.Logger.Debugf("DmsgService->SendMsg end")
	return request, nil
}

func (d *DmsgService) SetOnReceiveMsg(onReceiveMsg msg.OnReceiveMsg) {
	d.onReceiveMsg = onReceiveMsg
}

// sdk-custom-stream-protocol
func (d *DmsgService) RequestCustomStreamProtocol(peerIdEncode string, pid string, content []byte) error {
	protocolInfo := d.customStreamProtocolClientInfoList[pid]
	if protocolInfo == nil {
		dmsgLog.Logger.Errorf("DmsgService->RequestCustomStreamProtocol: protocol %s is not exist", pid)
		return fmt.Errorf("DmsgService->RequestCustomStreamProtocol: protocol %s is not exist", pid)
	}

	peerId, err := peer.Decode(peerIdEncode)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->RequestCustomStreamProtocol: err: %v", err)
		return err
	}
	_, _, err = protocolInfo.Protocol.Request(
		peerId,
		d.user.Key.PubkeyHex,
		pid,
		content)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->RequestCustomStreamProtocol: err: %v, servicePeerInfo: %v, user public key: %s, content: %v",
			err, peerId, d.user.Key.PubkeyHex, content)
		return err
	}
	return nil
}

func (d *DmsgService) RegistCustomStreamProtocolClient(client customProtocol.CustomStreamProtocolClient) error {
	customProtocolID := client.GetProtocolID()
	if d.customStreamProtocolClientInfoList[customProtocolID] != nil {
		dmsgLog.Logger.Errorf("DmsgService->RegistCustomStreamProtocolClient: protocol %s is already exist", customProtocolID)
		return fmt.Errorf("DmsgService->RegistCustomStreamProtocolClient: protocol %s is already exist", customProtocolID)
	}
	d.customStreamProtocolClientInfoList[customProtocolID] = &customProtocol.CustomStreamProtocolClientInfo{
		Protocol: adapter.NewCustomStreamProtocol(d.BaseService.GetCtx(), d.BaseService.GetHost(), customProtocolID, d, d),
		Client:   client,
	}
	client.SetCtx(d.BaseService.GetCtx())
	client.SetService(d)
	return nil
}

func (d *DmsgService) UnregistCustomStreamProtocolClient(client customProtocol.CustomStreamProtocolClient) error {
	customProtocolID := client.GetProtocolID()
	if d.customStreamProtocolClientInfoList[customProtocolID] == nil {
		dmsgLog.Logger.Warnf("DmsgService->UnregistCustomStreamProtocolClient: protocol %s is not exist", customProtocolID)
		return nil
	}
	d.customStreamProtocolClientInfoList[customProtocolID] = nil
	return nil
}

func (d *DmsgService) RegistCustomStreamProtocolService(service customProtocol.CustomStreamProtocolService) error {
	customProtocolID := service.GetProtocolID()
	if d.customStreamProtocolServiceInfoList[customProtocolID] != nil {
		dmsgLog.Logger.Errorf("dmsgService->RegistCustomStreamProtocol: protocol %s is already exist", customProtocolID)
		return fmt.Errorf("dmsgService->RegistCustomStreamProtocol: protocol %s is already exist", customProtocolID)
	}
	d.customStreamProtocolServiceInfoList[customProtocolID] = &customProtocol.CustomStreamProtocolServiceInfo{
		Protocol: adapter.NewCustomStreamProtocol(d.BaseService.GetCtx(), d.BaseService.GetHost(), customProtocolID, d, d),
		Service:  service,
	}
	service.SetCtx(d.BaseService.GetCtx())
	return nil
}

func (d *DmsgService) UnregistCustomStreamProtocolService(callback customProtocol.CustomStreamProtocolService) error {
	customProtocolID := callback.GetProtocolID()
	if d.customStreamProtocolServiceInfoList[customProtocolID] == nil {
		dmsgLog.Logger.Warnf("dmsgService->UnregistCustomStreamProtocol: protocol %s is not exist", customProtocolID)
		return nil
	}
	d.customStreamProtocolServiceInfoList[customProtocolID] = nil
	return nil
}

// ProtocolService interface
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

// StreamProtocolCallback interface
func (d *DmsgService) OnCreateMailboxRequest(
	requestProtoData protoreflect.ProtoMessage) (any, any, error) {
	dmsgLog.Logger.Debugf("dmsgService->OnCreateMailboxRequest begin:\nrequestProtoData: %+v", requestProtoData)
	request, ok := requestProtoData.(*pb.CreateMailboxReq)
	if !ok {
		dmsgLog.Logger.Errorf("dmsgService->OnCreateMailboxRequest: fail to convert requestProtoData to *pb.CreateMailboxReq")
		return nil, nil, fmt.Errorf("dmsgService->OnCreateMailboxRequest: fail to convert requestProtoData to *pb.CreateMailboxReq")
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
			Code:   dmsgProtocol.AlreadyExistCode,
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

func (d *DmsgService) OnCreateMailboxResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	dmsgLog.Logger.Debug("DmsgService->OnCreateMailboxResponse begin\nrequestProtoData: %+v\nresponseProtoData: %+v",
		requestProtoData, responseProtoData)
	request, ok := requestProtoData.(*pb.CreateMailboxReq)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnCreateMailboxResponse: fail to convert requestProtoData to *pb.CreateMailboxReq")
		return nil, fmt.Errorf("DmsgService->OnCreateMailboxResponse: fail to convert requestProtoData to *pb.CreateMailboxReq")
	}
	response, ok := responseProtoData.(*pb.CreateMailboxRes)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnCreateMailboxResponse: fail to convert responseProtoData to *pb.CreateMailboxRes")
		return nil, fmt.Errorf("DmsgService->OnCreateMailboxResponse: fail to convert responseProtoData to *pb.CreateMailboxRes")
	}

	switch response.RetCode.Code {
	case 0: // new
		fallthrough
	case 1: // exist mailbox
		dmsgLog.Logger.Debug("DmsgService->OnCreateMailboxResponse: mailbox has created, read message from mailbox...")
		err := d.releaseUnusedMailbox(response.BasicData.PeerID, request.BasicData.Pubkey)
		if err != nil {
			return nil, err
		}
	case -1:
		dmsgLog.Logger.Warnf("DmsgService->OnCreateMailboxResponse: fail RetCode: %+v ", response.RetCode)
	default:
		dmsgLog.Logger.Warnf("DmsgService->OnCreateMailboxResponse: other case RetCode: %+v", response.RetCode)
	}

	dmsgLog.Logger.Debug("DmsgService->OnCreateMailboxResponse end")
	return nil, nil
}

func (d *DmsgService) OnReleaseMailboxRequest(
	requestProtoData protoreflect.ProtoMessage) (any, any, error) {
	dmsgLog.Logger.Debugf("dmsgService->OnReleaseMailboxRequest begin:\nrequestProtoData: %+v", requestProtoData)
	request, ok := requestProtoData.(*pb.ReleaseMailboxReq)
	if !ok {
		dmsgLog.Logger.Errorf("dmsgService->OnReleaseMailboxRequest: fail to convert requestProtoData to *pb.ReleaseMailboxReq")
		return nil, nil, fmt.Errorf("dmsgService->OnReleaseMailboxRequest: fail to convert requestProtoData to *pb.ReleaseMailboxReq")
	}
	err := d.UnsubscribeDestUser(request.BasicData.Pubkey)
	if err != nil {
		return nil, nil, err
	}
	dmsgLog.Logger.Debugf("dmsgService->OnReleaseMailboxRequest end")
	return nil, nil, nil
}

func (d *DmsgService) OnReleaseMailboxResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	dmsgLog.Logger.Debug(
		"DmsgService->OnReleaseMailboxResponse begin\nrequestProtoData: %+v\nresponseProtoData: %+v",
		requestProtoData, responseProtoData)
	_, ok := requestProtoData.(*pb.ReleaseMailboxReq)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnReleaseMailboxResponse: fail to convert requestProtoData to *pb.ReleaseMailboxReq")
		return nil, fmt.Errorf("DmsgService->OnReleaseMailboxResponse: fail to convert requestProtoData to *pb.ReleaseMailboxReq")
	}

	response, ok := responseProtoData.(*pb.ReleaseMailboxRes)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnReleaseMailboxResponse: fail to convert responseProtoData to *pb.ReleaseMailboxRes")
		return nil, fmt.Errorf("DmsgService->OnReleaseMailboxResponse: fail to convert responseProtoData to *pb.ReleaseMailboxRes")
	}
	if response.RetCode.Code < 0 {
		dmsgLog.Logger.Warnf("DmsgService->OnReleaseMailboxResponse: fail RetCode: %+v", response.RetCode)
		return nil, fmt.Errorf("DmsgService->OnReleaseMailboxResponse: fail RetCode: %+v", response.RetCode)
	}

	dmsgLog.Logger.Debug("DmsgService->OnReleaseMailboxResponse end")
	return nil, nil
}

func (d *DmsgService) OnReadMailboxMsgRequest(requestProtoData protoreflect.ProtoMessage) (any, any, error) {
	dmsgLog.Logger.Debugf("dmsgService->OnReadMailboxMsgRequest begin:\nrequestProtoData: %+v", requestProtoData)
	request, ok := requestProtoData.(*pb.ReadMailboxReq)
	if !ok {
		dmsgLog.Logger.Errorf("dmsgService->OnReadMailboxMsgRequest: fail to convert requestProtoData to *pb.ReadMailboxReq")
		return nil, nil, fmt.Errorf("dmsgService->OnReadMailboxMsgRequest: fail to convert requestProtoData to *pb.ReadMailboxReq")
	}

	pubkey := request.BasicData.Pubkey
	user := d.GetDestUser(pubkey)
	if user == nil {
		dmsgLog.Logger.Errorf("dmsgService->OnReadMailboxMsgRequest: cannot find user for pubkey: %s", pubkey)
		return nil, nil, fmt.Errorf("dmsgService->OnReadMailboxMsgRequest: cannot find user for pubkey: %s", pubkey)
	}
	user.MsgRWMutex.RLock()
	defer user.MsgRWMutex.RUnlock()

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
		mailboxMsgDataList = append(mailboxMsgDataList, mailboxMsgData)
		needDeleteKeyList = append(needDeleteKeyList, string(result.Key))
		find = true
	}

	for _, needDeleteKey := range needDeleteKeyList {
		err := d.datastore.Delete(d.BaseService.GetCtx(), ds.NewKey(needDeleteKey))
		if err != nil {
			dmsgLog.Logger.Errorf("dmsgService->OnReadMailboxMsgRequest: datastore.Delete error: %+v", err)
		}
	}

	if !find {
		dmsgLog.Logger.Debug("dmsgService->OnReadMailboxMsgRequest: user msgs is empty")
	}
	dmsgLog.Logger.Debugf("dmsgService->OnReadMailboxMsgRequest end")
	return mailboxMsgDataList, nil, nil
}

func (d *DmsgService) OnReadMailboxMsgResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	dmsgLog.Logger.Debug("DmsgService->OnReadMailboxMsgResponse: begin\nrequestProtoData: %+v\nresponseProtoData: %+v",
		requestProtoData, responseProtoData)

	response, ok := responseProtoData.(*pb.ReadMailboxRes)
	if response == nil || !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnReadMailboxMsgResponse: fail to convert requestProtoData to *pb.ReadMailboxMsgRes")
		return nil, fmt.Errorf("DmsgService->OnReadMailboxMsgResponse: fail to convert requestProtoData to *pb.ReadMailboxMsgRes")
	}

	dmsgLog.Logger.Debugf("DmsgService->OnReadMailboxMsgResponse: found (%d) new message", len(response.ContentList))

	msgList, err := d.parseReadMailboxResponse(responseProtoData, msg.MsgDirection.From)
	if err != nil {
		return nil, err
	}
	for _, msg := range msgList {
		dmsgLog.Logger.Debugf("DmsgService->OnReadMailboxMsgResponse: From = %s, To = %s", msg.SrcPubkey, msg.DestPubkey)
		if d.onReceiveMsg != nil {
			d.onReceiveMsg(msg.SrcPubkey, msg.DestPubkey, msg.Content, msg.TimeStamp, msg.ID, msg.Direction)
		} else {
			dmsgLog.Logger.Errorf("DmsgService->OnReadMailboxMsgResponse: OnReceiveMsg is nil")
		}
	}

	dmsgLog.Logger.Debug("DmsgService->OnReadMailboxMsgResponse end")
	return nil, nil
}

func (d *DmsgService) OnCreateChannelRequest(
	requestProtoData protoreflect.ProtoMessage) (any, any, error) {
	dmsgLog.Logger.Debugf("dmsgService->OnCreateChannelRequest begin:\nrequestProtoData: %+v", requestProtoData)

	request, ok := requestProtoData.(*pb.CreateChannelReq)
	if !ok {
		dmsgLog.Logger.Errorf("dmsgService->OnCreateChannelRequest: fail to convert requestProtoData to *pb.CreateMailboxReq")
		return nil, nil, fmt.Errorf("dmsgService->OnCreateChannelRequest: cannot convert to *pb.CreateMailboxReq")
	}
	channelKey := request.ChannelKey
	isAvailable := d.isAvailablePubChannel(channelKey)
	if !isAvailable {
		return nil, nil, errors.New("dmsgService->OnCreateChannelRequest: exceeded the maximum number of mailbox service")
	}
	channel := d.channelList[channelKey]
	if channel != nil {
		dmsgLog.Logger.Errorf("dmsgService->OnCreateChannelRequest: channel already exist")
		retCode := &pb.RetCode{
			Code:   dmsgProtocol.AlreadyExistCode,
			Result: "dmsgService->OnCreateChannelRequest: channel already exist",
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

func (d *DmsgService) OnCreateChannelResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	dmsgLog.Logger.Debugf(
		"dmsgService->OnCreateChannelResponse begin:\nrequestProtoData: %+v\nresponseProtoData: %+v",
		requestProtoData, responseProtoData)

	dmsgLog.Logger.Debugf("dmsgService->OnCreateChannelResponse end")
	return nil, nil
}

func (d *DmsgService) OnCustomStreamProtocolRequest(
	requestProtoData protoreflect.ProtoMessage) (any, any, error) {
	dmsgLog.Logger.Debugf("dmsgService->OnCustomStreamProtocolRequest begin:\nrequestProtoData: %+v", requestProtoData)
	request, ok := requestProtoData.(*pb.CustomProtocolReq)
	if !ok {
		dmsgLog.Logger.Errorf("dmsgService->OnCustomStreamProtocolRequest: fail to convert requestProtoData to *pb.CustomContentReq")
		return nil, nil, fmt.Errorf("dmsgService->OnCustomStreamProtocolRequest: fail to convert requestProtoData to *pb.CustomContentReq")
	}

	customProtocolInfo := d.customStreamProtocolServiceInfoList[request.PID]
	if customProtocolInfo == nil {
		dmsgLog.Logger.Errorf("dmsgService->OnCustomStreamProtocolRequest: customProtocolInfo is nil, request: %+v", request)
		return nil, nil, fmt.Errorf("dmsgService->OnCustomStreamProtocolRequest: customProtocolInfo is nil, request: %+v", request)
	}
	err := customProtocolInfo.Service.HandleRequest(request)
	if err != nil {
		return nil, nil, err
	}
	param := &customProtocol.CustomStreamProtocolResponseParam{
		PID:     request.PID,
		Service: customProtocolInfo.Service,
	}

	dmsgLog.Logger.Debugf("dmsgService->OnCustomStreamProtocolRequest end")
	return param, nil, nil
}

func (d *DmsgService) OnCustomStreamProtocolResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	dmsgLog.Logger.Debugf(
		"dmsgService->OnCreateChannelResponse begin:\nrequestProtoData: %+v\nresponseProtoData: %+v",
		requestProtoData, responseProtoData)

	request, ok := requestProtoData.(*pb.CustomProtocolReq)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnCustomStreamProtocolResponse: fail to convert requestProtoData to *pb.CustomContentReq")
		return nil, fmt.Errorf("DmsgService->OnCustomStreamProtocolResponse: fail to convert requestProtoData to *pb.CustomContentReq")
	}
	response, ok := responseProtoData.(*pb.CustomProtocolRes)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnCustomStreamProtocolResponse: fail to convert requestProtoData to *pb.CustomContentRes")
		return nil, fmt.Errorf("DmsgService->OnCustomStreamProtocolResponse: fail to convert requestProtoData to *pb.CustomContentRes")
	}

	cfg := d.BaseService.GetConfig()
	switch cfg.Mode {
	case define.LightMode:

	}
	customProtocolInfo := d.customStreamProtocolClientInfoList[response.PID]
	if customProtocolInfo == nil {
		dmsgLog.Logger.Errorf("DmsgService->OnCustomStreamProtocolResponse: customProtocolInfo is nil, response: %+v", response)
		return nil, fmt.Errorf("DmsgService->OnCustomStreamProtocolResponse: customProtocolInfo is nil, response: %+v", response)
	}
	if customProtocolInfo.Client == nil {
		dmsgLog.Logger.Errorf("DmsgService->OnCustomStreamProtocolResponse: customProtocolInfo.Client is nil")
		return nil, fmt.Errorf("DmsgService->OnCustomStreamProtocolResponse: customProtocolInfo.Client is nil")
	}

	err := customProtocolInfo.Client.HandleResponse(request, response)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->OnCustomStreamProtocolResponse: Client.HandleResponse error: %v", err)
		return nil, err
	}
	return nil, nil
}

// PubsubProtocolCallback interface
func (d *DmsgService) OnSeekMailboxRequest(requestProtoData protoreflect.ProtoMessage) (any, any, error) {
	dmsgLog.Logger.Debug("DmsgService->OnSeekMailboxRequest begin\nrequestProtoData: %+v", requestProtoData)
	request, ok := requestProtoData.(*pb.SeekMailboxReq)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnCreateMailboxResponse: fail to convert requestProtoData to *pb.SeekMailboxReq")
		return nil, nil, fmt.Errorf("DmsgService->OnCreateMailboxResponse: fail to convert requestProtoData to *pb.SeekMailboxReq")
	}

	if request.BasicData.PeerID == d.BaseService.GetHost().ID().String() {
		dmsgLog.Logger.Debugf("dmsgService->OnReleaseMailboxRequest: request.BasicData.PeerID == d.BaseService.GetHost().ID")
		return nil, nil, fmt.Errorf("dmsgService->OnReleaseMailboxRequest: request.BasicData.PeerID == d.BaseService.GetHost().ID")
	}

	dmsgLog.Logger.Debug("DmsgService->OnSeekMailboxRequest end")
	return nil, nil, nil
}

func (d *DmsgService) OnSeekMailboxResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	dmsgLog.Logger.Debug(
		"DmsgService->OnSeekMailboxResponse begin\nrequestProtoData: %+v\nresponseProtoData: %+v",
		requestProtoData, responseProtoData)

	dmsgLog.Logger.Debug("DmsgService->OnSeekMailboxResponse end")
	return nil, nil
}

func (d *DmsgService) OnQueryPeerRequest(
	requestProtoData protoreflect.ProtoMessage) (any, any, error) {
	dmsgLog.Logger.Debug("DmsgService->OnQueryPeerRequest begin\nrequestProtoData: %+v", requestProtoData)
	// TODO implement it
	dmsgLog.Logger.Debug("DmsgService->OnQueryPeerRequest end")
	return nil, nil, nil
}

func (d *DmsgService) OnQueryPeerResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	dmsgLog.Logger.Debugf(
		"DmsgService->OnQueryPeerResponse begin\nrequestProtoData: %+v\nresponseProtoData: %+v",
		requestProtoData, responseProtoData)
	// TODO implement it
	dmsgLog.Logger.Debug("DmsgService->OnQueryPeerResponse end")
	return nil, nil
}

func (d *DmsgService) OnSendMsgRequest(
	requestProtoData protoreflect.ProtoMessage) (any, any, error) {
	dmsgLog.Logger.Debugf("dmsgService->OnSendMsgRequest begin:\nrequestProtoData: %+v", requestProtoData)
	request, ok := requestProtoData.(*pb.SendMsgReq)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnSendMsgRequest: cannot convert %v to *pb.SendMsgReq", requestProtoData)
		return nil, nil, fmt.Errorf("DmsgService->OnSendMsgRequest: cannot convert %v to *pb.SendMsgReq", requestProtoData)
	}

	cfg := d.BaseService.GetConfig()
	switch cfg.Mode {
	case define.ServiceMode:
		pubkey := request.BasicData.Pubkey
		user := d.GetDestUser(pubkey)
		if user == nil {
			dmsgLog.Logger.Errorf("dmsgService->OnSendMsgRequest: public key %s is not exist", pubkey)
			return nil, nil, fmt.Errorf("dmsgService->OnSendMsgRequest: public key %s is not exist", pubkey)
		}
		user.LastReciveTimestamp = time.Now().UnixNano()
		d.saveUserMsg(requestProtoData)
	case define.LightMode:
		if d.onReceiveMsg != nil {
			srcPubkey := request.BasicData.Pubkey
			destPubkey := request.DestPubkey
			msgDirection := msg.MsgDirection.From
			d.onReceiveMsg(
				srcPubkey,
				destPubkey,
				request.Content,
				request.BasicData.TS,
				request.BasicData.ID,
				msgDirection)
		} else {
			dmsgLog.Logger.Errorf("DmsgService->OnSendMsgRequest: OnReceiveMsg is nil")
		}
	}
	dmsgLog.Logger.Debugf("dmsgService->OnSendMsgRequest end")
	return nil, nil, nil
}

func (d *DmsgService) OnSendMsgResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	dmsgLog.Logger.Debugf(
		"dmsgService->OnSendMsgResponse begin:\nrequestProtoData: %+v\nresponseProtoData: %+v",
		requestProtoData, responseProtoData)
	response, ok := responseProtoData.(*pb.SendMsgRes)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnSendMsgResponse: cannot convert %v to *pb.SendMsgRes", responseProtoData)
		return nil, fmt.Errorf("DmsgService->OnSendMsgResponse: cannot convert %v to *pb.SendMsgRes", responseProtoData)
	}
	if response.RetCode.Code != 0 {
		dmsgLog.Logger.Warnf("DmsgService->OnSendMsgResponse: fail RetCode: %+v", response.RetCode)
		return nil, fmt.Errorf("DmsgService->OnSendMsgResponse: fail RetCode: %+v", response.RetCode)
	}
	dmsgLog.Logger.Debugf("dmsgService->OnSendMsgResponse end")
	return nil, nil
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

// mailbox
func (d *DmsgService) isAvailableMailbox(userPubKey string) bool {
	destUserCount := len(d.destUserList)
	cfg := d.BaseService.GetConfig()
	return destUserCount < cfg.DMsg.MaxMailboxPubsubCount
}

func (d *DmsgService) initMailbox(pubkey string) {
	dmsgLog.Logger.Debug("DmsgService->initMailbox begin")
	_, seekMailboxDoneChan, err := d.seekMailboxProtocol.Request(d.user.Key.PubkeyHex, d.user.Key.PubkeyHex)
	if err != nil {
		return
	}
	select {
	case seekMailboxResponseProtoData := <-seekMailboxDoneChan:
		dmsgLog.Logger.Debugf("DmsgService->initMailbox: seekMailboxProtoData: %+v", seekMailboxResponseProtoData)
		response, ok := seekMailboxResponseProtoData.(*pb.SeekMailboxRes)
		if !ok || response == nil {
			dmsgLog.Logger.Errorf("DmsgService->initMailbox: seekMailboxProtoData is not SeekMailboxRes")
			// skip seek when seek mailbox quest fail (server err), create a new mailbox
		}
		if response.RetCode.Code < 0 {
			dmsgLog.Logger.Errorf("DmsgService->initMailbox: seekMailboxProtoData fail")
			// skip seek when seek mailbox quest fail, create a new mailbox
		} else {
			dmsgLog.Logger.Debugf("DmsgService->initMailbox: seekMailboxProtoData success")
			go d.releaseUnusedMailbox(response.BasicData.PeerID, pubkey)
			return
		}
	case <-time.After(3 * time.Second):
		dmsgLog.Logger.Debugf("DmsgService->initMailbox: time.After 3s, create new mailbox")
		// begin create new mailbox

		hostId := d.BaseService.GetHost().ID().String()
		servicePeerList, err := d.BaseService.GetAvailableServicePeerList(hostId)
		if err != nil {
			dmsgLog.Logger.Errorf("DmsgService->initMailbox: getAvailableServicePeerList error: %v", err)
			return
		}

		peerID := d.BaseService.GetHost().ID().String()
		for _, servicePeerID := range servicePeerList {
			dmsgLog.Logger.Debugf("DmsgService->initMailbox: servicePeerID: %v", servicePeerID)
			if peerID == servicePeerID.String() {
				continue
			}
			_, createMailboxDoneChan, err := d.createMailboxProtocol.Request(servicePeerID, pubkey)
			if err != nil {
				dmsgLog.Logger.Errorf("DmsgService->initMailbox: createMailboxProtocol.Request error: %v", err)
				continue
			}

			select {
			case createMailboxResponseProtoData := <-createMailboxDoneChan:
				dmsgLog.Logger.Debugf("DmsgService->initMailbox: createMailboxResponseProtoData: %+v", createMailboxResponseProtoData)
				response, ok := createMailboxResponseProtoData.(*pb.CreateMailboxRes)
				if !ok || response == nil {
					dmsgLog.Logger.Errorf("DmsgService->initMailbox: createMailboxDoneChan is not CreateMailboxRes")
					continue
				}

				switch response.RetCode.Code {
				case 0, 1:
					dmsgLog.Logger.Debugf("DmsgService->initMailbox: createMailboxProtocol success")
					return
				default:
					continue
				}
			case <-time.After(time.Second * 3):
				continue
			case <-d.BaseService.GetCtx().Done():
				return
			}
		}

		dmsgLog.Logger.Error("DmsgService->InitUser: no available service peers")
		return
		// end create mailbox
	case <-d.BaseService.GetCtx().Done():
		dmsgLog.Logger.Debug("DmsgService->InitUser: BaseService.GetCtx().Done()")
		return
	}
}

func (d *DmsgService) releaseUnusedMailbox(peerIdHex string, pubkey string) error {
	dmsgLog.Logger.Debug("DmsgService->releaseUnusedMailbox begin")

	peerID, err := peer.Decode(peerIdHex)
	if err != nil {
		dmsgLog.Logger.Warnf("DmsgService->releaseUnusedMailbox: fail to decode peer id: %v", err)
		return err
	}
	_, readMailboxDoneChan, err := d.readMailboxMsgPrtocol.Request(peerID, pubkey)
	if err != nil {
		return err
	}

	select {
	case <-readMailboxDoneChan:
		if d.user.ServicePeerID == "" {
			d.user.ServicePeerID = peerIdHex
		} else if peerIdHex != d.user.ServicePeerID {
			_, releaseMailboxDoneChan, err := d.releaseMailboxPrtocol.Request(peerID, pubkey)
			if err != nil {
				return err
			}
			select {
			case <-releaseMailboxDoneChan:
				dmsgLog.Logger.Debugf("DmsgService->releaseUnusedMailbox: releaseMailboxDoneChan success")
			case <-time.After(time.Second * 3):
				return fmt.Errorf("DmsgService->releaseUnusedMailbox: releaseMailboxDoneChan time out")
			case <-d.BaseService.GetCtx().Done():
				return fmt.Errorf("DmsgService->releaseUnusedMailbox: BaseService.GetCtx().Done()")
			}
		}
	case <-time.After(time.Second * 10):
		return fmt.Errorf("DmsgService->releaseUnusedMailbox: readMailboxDoneChan time out")
	case <-d.BaseService.GetCtx().Done():
		return fmt.Errorf("DmsgService->releaseUnusedMailbox: BaseService.GetCtx().Done()")
	}
	dmsgLog.Logger.Debugf("DmsgService->releaseUnusedMailbox end")
	return nil
}

func (d *DmsgService) parseReadMailboxResponse(responseProtoData protoreflect.ProtoMessage, direction string) ([]msg.Msg, error) {
	dmsgLog.Logger.Debugf("DmsgService->parseReadMailboxResponse begin:\nresponseProtoData: %v", responseProtoData)
	msgList := []msg.Msg{}
	response, ok := responseProtoData.(*pb.ReadMailboxRes)
	if response == nil || !ok {
		dmsgLog.Logger.Errorf("DmsgService->parseReadMailboxResponse: fail to convert to *pb.ReadMailboxMsgRes")
		return msgList, fmt.Errorf("DmsgService->parseReadMailboxResponse: fail to convert to *pb.ReadMailboxMsgRes")
	}

	for _, mailboxItem := range response.ContentList {
		dmsgLog.Logger.Debugf("DmsgService->parseReadMailboxResponse: msg key = %s", mailboxItem.Key)
		msgContent := mailboxItem.Content

		fields := strings.Split(mailboxItem.Key, msg.MsgKeyDelimiter)
		if len(fields) < msg.MsgFieldsLen {
			dmsgLog.Logger.Errorf("DmsgService->parseReadMailboxResponse: msg key fields len not enough")
			return msgList, fmt.Errorf("DmsgService->parseReadMailboxResponse: msg key fields len not enough")
		}

		timeStamp, err := strconv.ParseInt(fields[msg.MsgTimeStampIndex], 10, 64)
		if err != nil {
			dmsgLog.Logger.Errorf("DmsgService->parseReadMailboxResponse: msg timeStamp parse error : %v", err)
			return msgList, fmt.Errorf("DmsgService->parseReadMailboxResponse: msg timeStamp parse error : %v", err)
		}

		destPubkey := fields[msg.MsgSrcUserPubKeyIndex]
		srcPubkey := fields[msg.MsgDestUserPubKeyIndex]
		msgID := fields[msg.MsgIDIndex]

		msgList = append(msgList, msg.Msg{
			ID:         msgID,
			SrcPubkey:  srcPubkey,
			DestPubkey: destPubkey,
			Content:    msgContent,
			TimeStamp:  timeStamp,
			Direction:  direction,
		})
	}
	dmsgLog.Logger.Debug("DmsgService->parseReadMailboxResponse end")
	return msgList, nil
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

func daysBetween(start, end int64) int {
	startTime := time.Unix(start, 0)
	endTime := time.Unix(end, 0)
	return int(endTime.Sub(startTime).Hours() / 24)
}
