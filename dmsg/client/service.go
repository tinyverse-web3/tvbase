package client

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	"github.com/libp2p/go-libp2p/core/peer"
	tvCommon "github.com/tinyverse-web3/tvbase/common"
	tvConfig "github.com/tinyverse-web3/tvbase/common/config"
	"github.com/tinyverse-web3/tvbase/common/db"
	"github.com/tinyverse-web3/tvbase/dmsg"
	dmsgClientCommon "github.com/tinyverse-web3/tvbase/dmsg/client/common"
	clientProtocol "github.com/tinyverse-web3/tvbase/dmsg/client/protocol"
	dmsgKey "github.com/tinyverse-web3/tvbase/dmsg/common/key"
	dmsgLog "github.com/tinyverse-web3/tvbase/dmsg/common/log"
	dmsgUser "github.com/tinyverse-web3/tvbase/dmsg/common/user"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
	customProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol/custom"
	tvutilKey "github.com/tinyverse-web3/tvutil/key"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type ProtocolProxy struct {
	readMailboxMsgPrtocol *dmsgClientCommon.StreamProtocol
	createMailboxProtocol *dmsgClientCommon.StreamProtocol
	releaseMailboxPrtocol *dmsgClientCommon.StreamProtocol
	createChannelProtocol *dmsgClientCommon.StreamProtocol
	seekMailboxProtocol   *dmsgClientCommon.PubsubProtocol
	queryPeerProtocol     *dmsgClientCommon.PubsubProtocol
	sendMsgPubPrtocol     *dmsgClientCommon.PubsubProtocol
}

type DmsgService struct {
	dmsg.DmsgService
	ProtocolProxy
	user         *dmsgUser.User
	onReceiveMsg dmsgClientCommon.OnReceiveMsg

	destUserList                 map[string]*dmsgUser.DestUser
	channelList                  map[string]*dmsgUser.Channel
	customStreamProtocolInfoList map[string]*dmsgClientCommon.CustomStreamProtocolInfo
	customPubsubProtocolInfoList map[string]*dmsgClientCommon.CustomPubsubProtocolInfo

	stopCleanRestResource chan bool
	// service
	datastore db.Datastore
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

	// stream protocol
	d.createMailboxProtocol = clientProtocol.NewCreateMailboxProtocol(d.BaseService.GetCtx(), d.BaseService.GetHost(), d, d)
	d.releaseMailboxPrtocol = clientProtocol.NewReleaseMailboxProtocol(d.BaseService.GetCtx(), d.BaseService.GetHost(), d, d)
	d.readMailboxMsgPrtocol = clientProtocol.NewReadMailboxMsgProtocol(d.BaseService.GetCtx(), d.BaseService.GetHost(), d, d)
	d.createChannelProtocol = clientProtocol.NewCreateChannelProtocol(d.BaseService.GetCtx(), d.BaseService.GetHost(), d, d)

	// pubsub protocol
	d.seekMailboxProtocol = clientProtocol.NewSeekMailboxProtocol(d.BaseService.GetCtx(), d.BaseService.GetHost(), d, d)
	d.RegPubsubProtocolResCallback(d.seekMailboxProtocol.Adapter.GetResponsePID(), d.seekMailboxProtocol)

	d.queryPeerProtocol = clientProtocol.NewQueryPeerProtocol(d.BaseService.GetCtx(), d.BaseService.GetHost(), d, d)
	d.RegPubsubProtocolReqCallback(d.queryPeerProtocol.Adapter.GetRequestPID(), d.queryPeerProtocol)

	d.sendMsgPubPrtocol = clientProtocol.NewSendMsgProtocol(d.BaseService.GetCtx(), d.BaseService.GetHost(), d, d)
	d.RegPubsubProtocolReqCallback(d.sendMsgPubPrtocol.Adapter.GetRequestPID(), d.sendMsgPubPrtocol)

	d.destUserList = make(map[string]*dmsgUser.DestUser)
	d.channelList = make(map[string]*dmsgUser.Channel)

	d.customStreamProtocolInfoList = make(map[string]*dmsgClientCommon.CustomStreamProtocolInfo)
	d.customPubsubProtocolInfoList = make(map[string]*dmsgClientCommon.CustomPubsubProtocolInfo)

	d.stopCleanRestResource = make(chan bool)
	return nil
}

// for sdk
func (d *DmsgService) Start() error {
	cfg := d.BaseService.GetConfig()
	if cfg.Mode == tvConfig.ServiceMode {
		d.cleanRestResource()
	}
	return nil
}

func (d *DmsgService) Stop() error {
	d.unsubscribeUser()
	d.UnSubscribeDestUserList()
	d.UnsubscribeChannelList()

	cfg := d.BaseService.GetConfig()
	if cfg.Mode == tvConfig.ServiceMode {
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
	if cfg.Mode == tvConfig.ServiceMode {
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

func (d *DmsgService) RequestReadMailbox(timeout time.Duration) ([]dmsg.Msg, error) {
	var msgList []dmsg.Msg
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
		msgList, err = d.parseReadMailboxResponse(response, dmsg.MsgDirection.From)
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

	dmsgLog.Logger.Debug("DmsgService->subscribeDestUser end")
	return nil
}

func (d *DmsgService) GetDestUser(pubkey string) *dmsgUser.DestUser {
	return d.destUserList[pubkey]
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

func (d *DmsgService) UnSubscribeDestUserList() {
	for pubkey := range d.destUserList {
		d.UnsubscribeDestUser(pubkey)
	}
}

func (d *DmsgService) SubscribeChannel(pubkey string) error {
	dmsgLog.Logger.Debugf("DmsgService->SubscribeChannel begin:\npubkey: %s", pubkey)

	if d.channelList[pubkey] != nil {
		dmsgLog.Logger.Errorf("DmsgService->SubscribeChannel: pubkey is already exist in pubChannelInfoList")
		return fmt.Errorf("DmsgService->SubscribeChannel: pubkey is already exist in pubChannelInfoList")
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

	err = d.createChannelService(channel.Key.PubkeyHex)
	if err != nil {
		target.Close()
		delete(d.channelList, pubkey)
		return err
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

func (d *DmsgService) UnsubscribeChannelList() {
	for pubkey := range d.channelList {
		d.UnsubscribeChannel(pubkey)
	}
}

func (d *DmsgService) SendMsg(destPubkey string, content []byte) (*pb.SendMsgReq, error) {
	dmsgLog.Logger.Debugf("DmsgService->SendMsg begin:\ndestPubkey: %v", destPubkey)
	sigPubkey := d.user.Key.PubkeyHex
	protoData, _, err := d.sendMsgPubPrtocol.Request(
		sigPubkey,
		destPubkey,
		content,
	)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->SendMsg: %v", err)
		return nil, err
	}
	sendMsgReq, ok := protoData.(*pb.SendMsgReq)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->SendMsg: protoData is not SendMsgReq")
		return nil, fmt.Errorf("DmsgService->SendMsg: protoData is not SendMsgReq")
	}
	dmsgLog.Logger.Debugf("DmsgService->SendMsg end")
	return sendMsgReq, nil
}

func (d *DmsgService) SetOnReceiveMsg(onReceiveMsg dmsgClientCommon.OnReceiveMsg) {
	d.onReceiveMsg = onReceiveMsg
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

// StreamProtocolCallback interface
func (d *DmsgService) OnCreateMailboxRequest(requestProtoData protoreflect.ProtoMessage) (any, error) {
	dmsgLog.Logger.Debug("DmsgService->OnCreateMailboxRequest begin\nrequestProtoData: %+v", requestProtoData)
	_, ok := requestProtoData.(*pb.CreateMailboxReq)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnCreateMailboxResponse: cannot convert %v to *pb.CreateMailboxReq", requestProtoData)
		return nil, fmt.Errorf("DmsgService->OnCreateMailboxResponse: cannot convert %v to *pb.CreateMailboxReq", requestProtoData)
	}

	dmsgLog.Logger.Debug("DmsgService->OnCreateMailboxRequest end")
	return nil, nil
}

func (d *DmsgService) OnCreateMailboxResponse(requestProtoData protoreflect.ProtoMessage, responseProtoData protoreflect.ProtoMessage) (any, error) {
	dmsgLog.Logger.Debug("DmsgService->OnCreateMailboxResponse begin\nrequestProtoData: %+v\nresponseProtoData: %+v",
		requestProtoData, responseProtoData)
	request, ok := requestProtoData.(*pb.CreateMailboxReq)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnCreateMailboxResponse: cannot convert %v to *pb.CreateMailboxReq", responseProtoData)
		return nil, fmt.Errorf("DmsgService->OnCreateMailboxResponse: cannot convert %v to *pb.CreateMailboxReq", responseProtoData)
	}
	response, ok := responseProtoData.(*pb.CreateMailboxRes)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnCreateMailboxResponse: cannot convert %v to *pb.CreateMailboxRes", responseProtoData)
		return nil, fmt.Errorf("DmsgService->OnCreateMailboxResponse: cannot convert %v to *pb.CreateMailboxRes", responseProtoData)
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

	default: // < 0 service no finish create mailbox
		dmsgLog.Logger.Warnf("DmsgService->OnCreateMailboxResponse: RetCode(%v) fail", response.RetCode)
	}

	dmsgLog.Logger.Debug("DmsgService->OnCreateMailboxResponse end")
	return nil, nil
}

func (d *DmsgService) OnCreateChannelRequest(requestProtoData protoreflect.ProtoMessage) (any, error) {
	dmsgLog.Logger.Debugf("DmsgService->OnCreateChannelRequest: begin\nrequestProtoData: %+v", requestProtoData)

	dmsgLog.Logger.Debugf("DmsgService->OnCreateChannelRequest end")
	return nil, nil
}

func (d *DmsgService) OnCreateChannelResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	dmsgLog.Logger.Debugf("dmsgService->OnCreateChannelResponse begin:\nrequestProtoData: %+v\nresponseProtoData: %+v",
		requestProtoData, responseProtoData)

	dmsgLog.Logger.Debugf("dmsgService->OnCreateChannelResponse end")
	return nil, nil
}

func (d *DmsgService) OnReleaseMailboxRequest(requestProtoData protoreflect.ProtoMessage) (any, error) {
	dmsgLog.Logger.Debugf("DmsgService->OnReleaseMailboxRequest: begin\nrequestProtoData: %+v", requestProtoData)

	dmsgLog.Logger.Debugf("DmsgService->OnReleaseMailboxRequest end")
	return nil, nil
}

func (d *DmsgService) OnReleaseMailboxResponse(requestProtoData protoreflect.ProtoMessage, responseProtoData protoreflect.ProtoMessage) (any, error) {
	dmsgLog.Logger.Debug("DmsgService->OnReleaseMailboxResponse begin")
	_, ok := requestProtoData.(*pb.ReleaseMailboxReq)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnCreateMailboxResponse: cannot convert %v to *pb.ReleaseMailboxReq", responseProtoData)
		return nil, fmt.Errorf("DmsgService->OnCreateMailboxResponse: cannot convert %v to *pb.ReleaseMailboxReq", responseProtoData)
	}

	response, ok := responseProtoData.(*pb.ReleaseMailboxRes)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnReleaseMailboxResponse: cannot convert %v to *pb.ReleaseMailboxRes", responseProtoData)
		return nil, fmt.Errorf("DmsgService->OnReleaseMailboxResponse: cannot convert %v to *pb.ReleaseMailboxRes", responseProtoData)
	}
	if response.RetCode.Code != 0 {
		dmsgLog.Logger.Warnf("DmsgService->OnReleaseMailboxResponse: RetCode(%v) fail", response.RetCode)
		return nil, fmt.Errorf("DmsgService->OnReleaseMailboxResponse: RetCode(%v) fail", response.RetCode)
	}

	dmsgLog.Logger.Debug("DmsgService->OnReleaseMailboxResponse end")
	return nil, nil
}

func (d *DmsgService) OnReadMailboxMsgRequest(requestProtoData protoreflect.ProtoMessage) (any, error) {
	dmsgLog.Logger.Debug("DmsgService->OnReadMailboxMsgRequest: begin\nrequestProtoData: %+v", requestProtoData)

	dmsgLog.Logger.Debugf("DmsgService->OnReadMailboxMsgRequest: end")
	return nil, nil
}

func (d *DmsgService) OnReadMailboxMsgResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	dmsgLog.Logger.Debug("DmsgService->OnReadMailboxMsgResponse: begin\nrequestProtoData: %+v\nresponseProtoData: %+v",
		requestProtoData, responseProtoData)

	response, ok := responseProtoData.(*pb.ReadMailboxRes)
	if response == nil || !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnReadMailboxMsgResponse: fail convert to *pb.ReadMailboxMsgRes")
		return nil, fmt.Errorf("DmsgService->OnReadMailboxMsgResponse: fail convert to *pb.ReadMailboxMsgRes")
	}

	dmsgLog.Logger.Debugf("DmsgService->OnReadMailboxMsgResponse: found (%d) new message", len(response.ContentList))

	msgList, err := d.parseReadMailboxResponse(responseProtoData, dmsg.MsgDirection.From)
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

// PubsubProtocolCallback interface
func (d *DmsgService) OnSeekMailboxRequest(requestProtoData protoreflect.ProtoMessage) (any, error) {
	dmsgLog.Logger.Debug("DmsgService->OnSeekMailboxRequest begin")

	dmsgLog.Logger.Debug("DmsgService->OnSeekMailboxRequest end")
	return nil, nil
}

func (d *DmsgService) OnSeekMailboxResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	dmsgLog.Logger.Debug("DmsgService->OnSeekMailboxResponse begin")

	dmsgLog.Logger.Debugf("DmsgService->OnSeekMailboxResponse end")
	return nil, nil
}

func (d *DmsgService) OnQueryPeerRequest(requestProtoData protoreflect.ProtoMessage) (any, error) {
	// TODO implement it
	return nil, nil
}

func (d *DmsgService) OnQueryPeerResponse(requestProtoData protoreflect.ProtoMessage, responseProtoData protoreflect.ProtoMessage) (any, error) {
	// TODO implement it
	return nil, nil
}

func (d *DmsgService) OnSendMsgRequest(requestProtoData protoreflect.ProtoMessage) (any, error) {
	sendMsgReq, ok := requestProtoData.(*pb.SendMsgReq)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnSendMsgRequest: cannot convert %v to *pb.SendMsgReq", requestProtoData)
		return nil, fmt.Errorf("DmsgService->OnSendMsgRequest: cannot convert %v to *pb.SendMsgReq", requestProtoData)
	}

	if d.onReceiveMsg != nil {
		srcPubkey := sendMsgReq.BasicData.Pubkey
		destPubkey := sendMsgReq.DestPubkey
		msgDirection := dmsg.MsgDirection.From
		d.onReceiveMsg(
			srcPubkey,
			destPubkey,
			sendMsgReq.Content,
			sendMsgReq.BasicData.TS,
			sendMsgReq.BasicData.ID,
			msgDirection)
	} else {
		dmsgLog.Logger.Errorf("DmsgService->OnSendMsgRequest: OnReceiveMsg is nil")
	}

	return nil, nil
}

func (d *DmsgService) OnSendMsgResponse(requestProtoData protoreflect.ProtoMessage, responseProtoData protoreflect.ProtoMessage) (any, error) {
	response, ok := responseProtoData.(*pb.SendMsgRes)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnSendMsgResponse: cannot convert %v to *pb.SendMsgRes", responseProtoData)
		return nil, fmt.Errorf("DmsgService->OnSendMsgResponse: cannot convert %v to *pb.SendMsgRes", responseProtoData)
	}
	if response.RetCode.Code != 0 {
		dmsgLog.Logger.Warnf("DmsgService->OnSendMsgResponse: RetCode(%v) fail", response.RetCode)
		return nil, fmt.Errorf("DmsgService->OnSendMsgResponse: RetCode(%v) fail", response.RetCode)
	}
	return nil, nil
}

func (d *DmsgService) OnCustomStreamProtocolRequest(requestProtoData protoreflect.ProtoMessage) (any, error) {
	return nil, nil
}

func (d *DmsgService) OnCustomStreamProtocolResponse(requestProtoData protoreflect.ProtoMessage, responseProtoData protoreflect.ProtoMessage) (any, error) {
	request, ok := requestProtoData.(*pb.CustomProtocolReq)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnCustomStreamProtocolResponse: cannot convert %v to *pb.CustomContentRes", requestProtoData)
		return nil, fmt.Errorf("DmsgService->OnCustomStreamProtocolResponse: cannot convert %v to *pb.CustomContentRes", requestProtoData)
	}
	response, ok := responseProtoData.(*pb.CustomProtocolRes)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnCustomStreamProtocolResponse: cannot convert %v to *pb.CustomContentRes", responseProtoData)
		return nil, fmt.Errorf("DmsgService->OnCustomStreamProtocolResponse: cannot convert %v to *pb.CustomContentRes", responseProtoData)
	}

	customProtocolInfo := d.customStreamProtocolInfoList[response.PID]
	if customProtocolInfo == nil {
		dmsgLog.Logger.Errorf("DmsgService->OnCustomStreamProtocolResponse: customProtocolInfo(%v) is nil", response.PID)
		return nil, fmt.Errorf("DmsgService->OnCustomStreamProtocolResponse: customProtocolInfo(%v) is nil", customProtocolInfo)
	}
	if customProtocolInfo.Client == nil {
		dmsgLog.Logger.Errorf("DmsgService->OnCustomStreamProtocolResponse: Client is nil")
		return nil, fmt.Errorf("DmsgService->OnCustomStreamProtocolResponse: Client is nil")
	}

	err := customProtocolInfo.Client.HandleResponse(request, response)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->OnCustomStreamProtocolResponse: HandleResponse error: %v", err)
		return nil, err
	}
	return nil, nil
}

func (d *DmsgService) OnCustomPubsubProtocolRequest(requestProtoData protoreflect.ProtoMessage) (any, error) {
	return nil, nil
}

func (d *DmsgService) OnCustomPubsubProtocolResponse(requestProtoData protoreflect.ProtoMessage, responseProtoData protoreflect.ProtoMessage) (any, error) {
	request, ok := requestProtoData.(*pb.CustomProtocolReq)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnCustomPubsubProtocolResponse: cannot convert %v to *pb.CustomContentRes", requestProtoData)
		return nil, fmt.Errorf("DmsgService->OnCustomPubsubProtocolResponse: cannot convert %v to *pb.CustomContentRes", requestProtoData)
	}
	response, ok := responseProtoData.(*pb.CustomProtocolRes)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnCustomPubsubProtocolResponse: cannot convert %v to *pb.CustomContentRes", responseProtoData)
		return nil, fmt.Errorf("DmsgService->OnCustomPubsubProtocolResponse: cannot convert %v to *pb.CustomContentRes", responseProtoData)
	}
	customPubsubProtocolInfo := d.customPubsubProtocolInfoList[response.PID]
	if customPubsubProtocolInfo == nil {
		dmsgLog.Logger.Errorf("DmsgService->OnCustomPubsubProtocolResponse: customProtocolInfo(%v) is nil", response.PID)
		return nil, fmt.Errorf("DmsgService->OnCustomPubsubProtocolResponse: customProtocolInfo(%v) is nil", customPubsubProtocolInfo)
	}
	if customPubsubProtocolInfo.Client == nil {
		dmsgLog.Logger.Errorf("DmsgService->OnCustomPubsubProtocolResponse: Client is nil")
		return nil, fmt.Errorf("DmsgService->OnCustomPubsubProtocolResponse: Client is nil")
	}

	err := customPubsubProtocolInfo.Client.HandleResponse(request, response)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->OnCustomPubsubProtocolResponse: HandleResponse error: %v", err)
		return nil, err
	}
	return nil, nil
}

// ClientService interface
func (d *DmsgService) PublishProtocol(pubkey string, pid pb.PID, protoData []byte) error {
	var dmsguser *dmsgUser.Target = nil
	user := d.destUserList[pubkey]
	if user == nil {
		if d.user.Key.PubkeyHex != pubkey {
			channel := d.channelList[pubkey]
			if channel == nil {
				dmsgLog.Logger.Errorf("DmsgService->PublishProtocol: cannot find src/dest/channel user Info for key %s", pubkey)
				return fmt.Errorf("DmsgService->PublishProtocol: cannot find src/dest/channel user Info for key %s", pubkey)
			}
			dmsguser = &channel.Target
		} else {
			dmsguser = &d.user.Target
		}
	} else {
		dmsguser = &user.Target
	}

	buf, err := dmsgProtocol.GenProtoData(pid, protoData)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->PublishProtocol: GenProtoData error: %v", err)
		return err
	}

	err = dmsguser.Publish(buf)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->PublishProtocol: Publish error: %v", err)
		return err
	}
	return nil
}

// cmd protocol
func (d *DmsgService) RequestCustomStreamProtocol(peerIdEncode string, pid string, content []byte) error {
	protocolInfo := d.customStreamProtocolInfoList[pid]
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

func (d *DmsgService) RegistCustomStreamProtocol(client customProtocol.CustomStreamProtocolClient) error {
	customProtocolID := client.GetProtocolID()
	if d.customStreamProtocolInfoList[customProtocolID] != nil {
		dmsgLog.Logger.Errorf("DmsgService->RegistCustomStreamProtocol: protocol %s is already exist", customProtocolID)
		return fmt.Errorf("DmsgService->RegistCustomStreamProtocol: protocol %s is already exist", customProtocolID)
	}
	d.customStreamProtocolInfoList[customProtocolID] = &dmsgClientCommon.CustomStreamProtocolInfo{
		Protocol: clientProtocol.NewCustomStreamProtocol(d.BaseService.GetCtx(), d.BaseService.GetHost(), customProtocolID, d, d),
		Client:   client,
	}
	client.SetCtx(d.BaseService.GetCtx())
	client.SetService(d)
	return nil
}

func (d *DmsgService) UnregistCustomStreamProtocol(client customProtocol.CustomStreamProtocolClient) error {
	customProtocolID := client.GetProtocolID()
	if d.customStreamProtocolInfoList[customProtocolID] == nil {
		dmsgLog.Logger.Warnf("DmsgService->UnregistCustomStreamProtocol: protocol %s is not exist", customProtocolID)
		return nil
	}
	d.customStreamProtocolInfoList[customProtocolID] = nil
	return nil
}

func (d *DmsgService) RegistCustomPubsubProtocol(client customProtocol.CustomPubsubProtocolClient) error {
	customProtocolID := client.GetProtocolID()
	if d.customPubsubProtocolInfoList[customProtocolID] != nil {
		dmsgLog.Logger.Errorf("DmsgService->RegistCustomPubsubProtocol: protocol %s is already exist", customProtocolID)
		return fmt.Errorf("DmsgService->RegistCustomPubsubProtocol: protocol %s is already exist", customProtocolID)
	}
	d.customPubsubProtocolInfoList[customProtocolID] = &dmsgClientCommon.CustomPubsubProtocolInfo{
		Protocol: clientProtocol.NewCustomPubsubProtocol(d.BaseService.GetCtx(), d.BaseService.GetHost(), d, d),
		Client:   client,
	}
	client.SetCtx(d.BaseService.GetCtx())
	client.SetService(d)
	return nil
}

func (d *DmsgService) UnregistCustomPubsubProtocol(client customProtocol.CustomPubsubProtocolClient) error {
	customProtocolID := client.GetProtocolID()
	if d.customPubsubProtocolInfoList[customProtocolID] == nil {
		dmsgLog.Logger.Warnf("DmsgService->UnregistCustomPubsubProtocol: protocol %s is not exist", customProtocolID)
		return nil
	}
	d.customPubsubProtocolInfoList[customProtocolID] = nil
	return nil
}

// user
func (d *DmsgService) subscribeUser(pubkey string, getSig dmsgKey.GetSigCallback) error {
	dmsgLog.Logger.Debugf("DmsgService->subscribeUser begin\npubkey: %s", pubkey)
	if d.user != nil {
		dmsgLog.Logger.Errorf("DmsgService->subscribeUser: user isn't nil")
		return fmt.Errorf("DmsgService->subscribeUser: user isn't nil")
	}

	target, err := dmsgUser.NewTarget(d.BaseService.GetCtx(), d.Pubsub, pubkey, getSig)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->subscribeUser: NewTarget error: %v", err)
		return err
	}
	d.user = &dmsgUser.User{
		Target:        *target,
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

// mailbox
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

func (d *DmsgService) parseReadMailboxResponse(responseProtoData protoreflect.ProtoMessage, direction string) ([]dmsg.Msg, error) {
	dmsgLog.Logger.Debugf("DmsgService->parseReadMailboxResponse begin:\nresponseProtoData: %v", responseProtoData)
	msgList := []dmsg.Msg{}
	response, ok := responseProtoData.(*pb.ReadMailboxRes)
	if response == nil || !ok {
		dmsgLog.Logger.Errorf("DmsgService->parseReadMailboxResponse: fail to convert to *pb.ReadMailboxMsgRes")
		return msgList, fmt.Errorf("DmsgService->parseReadMailboxResponse: fail to convert to *pb.ReadMailboxMsgRes")
	}

	for _, mailboxItem := range response.ContentList {
		dmsgLog.Logger.Debugf("DmsgService->parseReadMailboxResponse: msg key = %s", mailboxItem.Key)
		msgContent := mailboxItem.Content

		fields := strings.Split(mailboxItem.Key, dmsg.MsgKeyDelimiter)
		if len(fields) < dmsg.MsgFieldsLen {
			dmsgLog.Logger.Errorf("DmsgService->parseReadMailboxResponse: msg key fields len not enough")
			return msgList, fmt.Errorf("DmsgService->parseReadMailboxResponse: msg key fields len not enough")
		}

		timeStamp, err := strconv.ParseInt(fields[dmsg.MsgTimeStampIndex], 10, 64)
		if err != nil {
			dmsgLog.Logger.Errorf("DmsgService->parseReadMailboxResponse: msg timeStamp parse error : %v", err)
			return msgList, fmt.Errorf("DmsgService->parseReadMailboxResponse: msg timeStamp parse error : %v", err)
		}

		destPubkey := fields[dmsg.MsgSrcUserPubKeyIndex]
		srcPubkey := fields[dmsg.MsgDestUserPubKeyIndex]
		msgID := fields[dmsg.MsgIDIndex]

		msgList = append(msgList, dmsg.Msg{
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
	for _, servicePeerID := range servicePeerList {
		dmsgLog.Logger.Debugf("DmsgService->createChannelService: servicePeerID: %s", servicePeerID)
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
				for channelPk, pubsub := range d.channelList {
					days := daysBetween(pubsub.LastReciveTimestamp, time.Now().UnixNano())
					// delete mailbox msg in datastore and cancel mailbox subscribe when days is over, default days is 30
					if days >= cfg.DMsg.KeepPubChannelDay {
						d.UnsubscribeChannel(channelPk)
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
