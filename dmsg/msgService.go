package dmsg

import (
	"errors"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	tvbaseCommon "github.com/tinyverse-web3/tvbase/common"
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

type MsgService struct {
	Service
	createChannelProtocol               *dmsgProtocol.MsgSProtocol
	sendMsgProtocol                     *dmsgProtocol.MsgPProtocol
	srcUser                             *dmsgUser.User
	onReceiveMsg                        msg.OnReceiveMsg
	destUserList                        map[string]*dmsgUser.LightMsgUser
	channelList                         map[string]*dmsgUser.Channel
	customStreamProtocolServiceInfoList map[string]*customProtocol.CustomStreamProtocolServiceInfo
	customStreamProtocolClientInfoList  map[string]*customProtocol.CustomStreamProtocolClientInfo
	stopCleanRestResource               chan bool
}

func CreateDmsgService(tvbaseService tvbaseCommon.TvBaseService) (*MsgService, error) {
	d := &MsgService{}
	err := d.Init(tvbaseService)
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (d *MsgService) Init(tvbaseService tvbaseCommon.TvBaseService) error {
	err := d.Service.Init(tvbaseService)
	if err != nil {
		return err
	}

	d.destUserList = make(map[string]*dmsgUser.LightMsgUser)
	d.channelList = make(map[string]*dmsgUser.Channel)

	d.customStreamProtocolClientInfoList = make(map[string]*customProtocol.CustomStreamProtocolClientInfo)
	d.customStreamProtocolServiceInfoList = make(map[string]*customProtocol.CustomStreamProtocolServiceInfo)

	return nil
}

// sdk-common
func (d *MsgService) Start(enableService bool, pubkeyData []byte, getSig dmsgKey.GetSigCallback, opts ...any) error {
	dmsgLog.Logger.Debug("DmsgService->Start begin")
	err := d.Service.Start(enableService)
	if err != nil {
		return err
	}
	if d.enableService {
		d.stopCleanRestResource = make(chan bool)
		d.cleanRestResource()
	}

	ctx := d.baseService.GetCtx()
	host := d.baseService.GetHost()
	// stream protocol
	d.createChannelProtocol = adapter.NewCreateChannelProtocol(ctx, host, d, d)

	// pubsub protocol
	d.sendMsgProtocol = adapter.NewSendMsgProtocol(ctx, host, d, d)
	d.registPubsubProtocol(d.sendMsgProtocol.Adapter.GetRequestPID(), d.sendMsgProtocol)

	// user
	d.initSrcUser(pubkeyData, getSig, opts...)

	dmsgLog.Logger.Debug("DmsgService->Start end")
	return nil
}

func (d *MsgService) Stop() error {
	dmsgLog.Logger.Debug("DmsgService->Stop begin")
	err := d.Service.Stop()
	if err != nil {
		return err
	}

	d.unsubscribeSrcUser()
	d.UnsubscribeDestUserList()
	d.UnsubscribeChannelList()
	d.stopCleanRestResource <- true
	close(d.stopCleanRestResource)
	dmsgLog.Logger.Debug("DmsgService->Stop end")
	return nil
}

// sdk-destuser
func (d *MsgService) GetDestUser(pubkey string) *dmsgUser.LightMsgUser {
	return d.destUserList[pubkey]
}

func (d *MsgService) SubscribeDestUser(pubkey string) error {
	dmsgLog.Logger.Debug("DmsgService->SubscribeDestUser begin\npubkey: %s", pubkey)
	if d.destUserList[pubkey] != nil {
		dmsgLog.Logger.Errorf("DmsgService->SubscribeDestUser: pubkey is already exist in destUserList")
		return fmt.Errorf("DmsgService->SubscribeDestUser: pubkey is already exist in destUserList")
	}
	target, err := dmsgUser.NewTarget(d.baseService.GetCtx(), pubkey, nil)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->SubscribeDestUser: NewTarget error: %v", err)
		return err
	}
	err = target.InitPubsub(d.pubsub, pubkey)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->subscribeUser: InitPubsub error: %v", err)
		return err
	}

	user := &dmsgUser.LightMsgUser{
		Target: *target,
	}

	d.destUserList[pubkey] = user
	// go d.BaseService.DiscoverRendezvousPeers()
	return nil
}

func (d *MsgService) UnsubscribeDestUser(pubkey string) error {
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

func (d *MsgService) UnsubscribeDestUserList() error {
	for userPubKey := range d.destUserList {
		d.UnsubscribeDestUser(userPubKey)
	}
	return nil
}

// sdk-channel
func (d *MsgService) SubscribeChannel(pubkey string) error {
	dmsgLog.Logger.Debugf("DmsgService->SubscribeChannel begin:\npubkey: %s", pubkey)

	if d.channelList[pubkey] != nil {
		dmsgLog.Logger.Errorf("DmsgService->SubscribeChannel: pubkey is already exist in channelList")
		return fmt.Errorf("DmsgService->SubscribeChannel: pubkey is already exist in channelList")
	}

	target, err := dmsgUser.NewTarget(d.baseService.GetCtx(), pubkey, nil)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->SubscribeChannel: NewTarget error: %v", err)
		return err
	}

	err = target.InitPubsub(d.pubsub, pubkey)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->subscribeUser: InitPubsub error: %v", err)
		return err
	}

	channel := &dmsgUser.Channel{
		DestTarget: dmsgUser.DestTarget{
			Target:              *target,
			LastReciveTimestamp: time.Now().UnixNano(),
		},
	}
	d.channelList[pubkey] = channel

	if !d.enableService {
		err = d.createChannelService(channel.Key.PubkeyHex)
		if err != nil {
			target.Close()
			delete(d.channelList, pubkey)
			return err
		}
	}

	// go d.BaseService.DiscoverRendezvousPeers()
	go d.handlePubsubProtocol(&channel.Target)

	dmsgLog.Logger.Debug("DmsgService->SubscribeChannel end")
	return nil
}

func (d *MsgService) UnsubscribeChannel(pubkey string) error {
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

func (d *MsgService) UnsubscribeChannelList() error {
	for pubkey := range d.channelList {
		d.UnsubscribeChannel(pubkey)
	}
	return nil
}

// sdk-msg
func (d *MsgService) SendMsg(destPubkey string, content []byte) (*pb.SendMsgReq, error) {
	dmsgLog.Logger.Debugf("DmsgService->SendMsg begin:\ndestPubkey: %s", destPubkey)
	requestProtoData, _, err := d.sendMsgProtocol.Request(d.srcUser.Key.PubkeyHex, destPubkey, content)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->SendMsg: sendMsgProtocol.Request error: %v", err)
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

func (d *MsgService) SetOnReceiveMsg(onReceiveMsg msg.OnReceiveMsg) {
	d.onReceiveMsg = onReceiveMsg
}

// sdk-custom-stream-protocol
func (d *MsgService) RequestCustomStreamProtocol(peerIdEncode string, pid string, content []byte) error {
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
		d.srcUser.Key.PubkeyHex,
		pid,
		content)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->RequestCustomStreamProtocol: err: %v, servicePeerInfo: %v, user public key: %s, content: %v",
			err, peerId, d.srcUser.Key.PubkeyHex, content)
		return err
	}
	return nil
}

func (d *MsgService) RegistCustomStreamProtocolClient(client customProtocol.CustomStreamProtocolClient) error {
	customProtocolID := client.GetProtocolID()
	if d.customStreamProtocolClientInfoList[customProtocolID] != nil {
		dmsgLog.Logger.Errorf("DmsgService->RegistCustomStreamProtocolClient: protocol %s is already exist", customProtocolID)
		return fmt.Errorf("DmsgService->RegistCustomStreamProtocolClient: protocol %s is already exist", customProtocolID)
	}
	d.customStreamProtocolClientInfoList[customProtocolID] = &customProtocol.CustomStreamProtocolClientInfo{
		Protocol: adapter.NewCustomStreamProtocol(d.baseService.GetCtx(), d.baseService.GetHost(), customProtocolID, d, d),
		Client:   client,
	}
	client.SetCtx(d.baseService.GetCtx())
	client.SetService(d)
	return nil
}

func (d *MsgService) UnregistCustomStreamProtocolClient(client customProtocol.CustomStreamProtocolClient) error {
	customProtocolID := client.GetProtocolID()
	if d.customStreamProtocolClientInfoList[customProtocolID] == nil {
		dmsgLog.Logger.Warnf("DmsgService->UnregistCustomStreamProtocolClient: protocol %s is not exist", customProtocolID)
		return nil
	}
	d.customStreamProtocolClientInfoList[customProtocolID] = nil
	return nil
}

func (d *MsgService) RegistCustomStreamProtocolService(service customProtocol.CustomStreamProtocolService) error {
	customProtocolID := service.GetProtocolID()
	if d.customStreamProtocolServiceInfoList[customProtocolID] != nil {
		dmsgLog.Logger.Errorf("dmsgService->RegistCustomStreamProtocol: protocol %s is already exist", customProtocolID)
		return fmt.Errorf("dmsgService->RegistCustomStreamProtocol: protocol %s is already exist", customProtocolID)
	}
	d.customStreamProtocolServiceInfoList[customProtocolID] = &customProtocol.CustomStreamProtocolServiceInfo{
		Protocol: adapter.NewCustomStreamProtocol(d.baseService.GetCtx(), d.baseService.GetHost(), customProtocolID, d, d),
		Service:  service,
	}
	service.SetCtx(d.baseService.GetCtx())
	return nil
}

func (d *MsgService) UnregistCustomStreamProtocolService(callback customProtocol.CustomStreamProtocolService) error {
	customProtocolID := callback.GetProtocolID()
	if d.customStreamProtocolServiceInfoList[customProtocolID] == nil {
		dmsgLog.Logger.Warnf("dmsgService->UnregistCustomStreamProtocol: protocol %s is not exist", customProtocolID)
		return nil
	}
	d.customStreamProtocolServiceInfoList[customProtocolID] = nil
	return nil
}

// DmsgServiceInterface
func (d *MsgService) GetUserPubkeyHex() (string, error) {
	if d.srcUser == nil {
		return "", fmt.Errorf("DmsgService->GetUserPubkeyHex: user is nil")
	}
	return d.srcUser.Key.PubkeyHex, nil
}

func (d *MsgService) GetUserSig(protoData []byte) ([]byte, error) {
	if d.srcUser == nil {
		dmsgLog.Logger.Errorf("DmsgService->GetUserSig: user is nil")
		return nil, fmt.Errorf("DmsgService->GetUserSig: user is nil")
	}
	return d.srcUser.GetSig(protoData)
}

func (d *MsgService) GetPublishTarget(pubkey string) *dmsgUser.Target {
	var target *dmsgUser.Target = nil
	user := d.destUserList[pubkey]
	if user == nil {
		if d.srcUser.Key.PubkeyHex != pubkey {
			channel := d.channelList[pubkey]
			if channel == nil {
				return nil
			}
			target = &channel.Target
		} else {
			target = &d.srcUser.Target
		}
	} else {
		target = &user.Target
	}
	return target
}

// MsgSpCallback
func (d *MsgService) OnCreateChannelRequest(
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

func (d *MsgService) OnCreateChannelResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	dmsgLog.Logger.Debugf(
		"dmsgService->OnCreateChannelResponse begin:\nrequestProtoData: %+v\nresponseProtoData: %+v",
		requestProtoData, responseProtoData)

	dmsgLog.Logger.Debugf("dmsgService->OnCreateChannelResponse end")
	return nil, nil
}

func (d *MsgService) OnCustomStreamProtocolRequest(
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

func (d *MsgService) OnCustomStreamProtocolResponse(
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

// MsgPpCallback
func (d *MsgService) OnQueryPeerRequest(
	requestProtoData protoreflect.ProtoMessage) (any, any, error) {
	dmsgLog.Logger.Debug("DmsgService->OnQueryPeerRequest begin\nrequestProtoData: %+v", requestProtoData)
	// TODO implement it
	dmsgLog.Logger.Debug("DmsgService->OnQueryPeerRequest end")
	return nil, nil, nil
}

func (d *MsgService) OnQueryPeerResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	dmsgLog.Logger.Debugf(
		"DmsgService->OnQueryPeerResponse begin\nrequestProtoData: %+v\nresponseProtoData: %+v",
		requestProtoData, responseProtoData)
	// TODO implement it
	dmsgLog.Logger.Debug("DmsgService->OnQueryPeerResponse end")
	return nil, nil
}

func (d *MsgService) OnSendMsgRequest(
	requestProtoData protoreflect.ProtoMessage) (any, any, error) {
	dmsgLog.Logger.Debugf("dmsgService->OnSendMsgRequest begin:\nrequestProtoData: %+v", requestProtoData)
	request, ok := requestProtoData.(*pb.SendMsgReq)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnSendMsgRequest: fail to convert requestProtoData to *pb.SendMsgReq")
		return nil, nil, fmt.Errorf("DmsgService->OnSendMsgRequest: fail to convert requestProtoData to *pb.SendMsgReq")
	}

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

	dmsgLog.Logger.Debugf("dmsgService->OnSendMsgRequest end")
	return nil, nil, nil
}

func (d *MsgService) OnSendMsgResponse(
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
func (d *MsgService) cleanRestResource() {
	go func() {
		ticker := time.NewTicker(3 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-d.stopCleanRestResource:
				return
			case <-ticker.C:
				for pubChannelPubkey, pubsub := range d.channelList {
					days := daysBetween(pubsub.LastReciveTimestamp, time.Now().UnixNano())
					if days >= d.GetConfig().KeepPubChannelDay {
						d.UnsubscribeChannel(pubChannelPubkey)
						return
					}
				}

				continue
			case <-d.baseService.GetCtx().Done():
				return
			}
		}
	}()
}

func (d *MsgService) handlePubsubProtocol(target *dmsgUser.Target) {
	for {
		m, err := target.WaitMsg()
		if err != nil {
			dmsgLog.Logger.Warnf("dmsgService->handlePubsubProtocol: target.WaitMsg error: %+v", err)
			return
		}

		dmsgLog.Logger.Debugf("dmsgService->handlePubsubProtocol:\ntopic: %s\nreceivedFrom: %+v", m.Topic, m.ReceivedFrom)

		protocolID, protocolIDLen, err := d.checkProtocolData(m.Data)
		if err != nil {
			dmsgLog.Logger.Errorf("dmsgService->handlePubsubProtocol: CheckPubsubData error: %v", err)
			continue
		}
		protocolData := m.Data[protocolIDLen:]
		protocolHandle := d.protocolHandleList[protocolID]
		if protocolHandle != nil {
			err = protocolHandle.HandleRequestData(protocolData)
			if err != nil {
				dmsgLog.Logger.Warnf("dmsgService->handlePubsubProtocol: HandleRequestData error: %v", err)
			}
			continue
		} else {
			dmsgLog.Logger.Warnf("dmsgService->handlePubsubProtocol: no protocolHandle for protocolID: %d", protocolID)
		}
	}
}

// src user
func (d *MsgService) initSrcUser(pubkeyData []byte, getSig dmsgKey.GetSigCallback, opts ...any) error {
	dmsgLog.Logger.Debug("DmsgService->InitUser begin")
	pubkey := tvutilKey.TranslateKeyProtoBufToString(pubkeyData)
	err := d.subscribeSrcUser(pubkey, getSig)
	if err != nil {
		return err
	}

	dmsgLog.Logger.Debug("DmsgService->InitUser end")
	return nil
}

func (d *MsgService) subscribeSrcUser(pubkey string, getSig dmsgKey.GetSigCallback) error {
	dmsgLog.Logger.Debugf("DmsgService->subscribeUser begin\npubkey: %s", pubkey)
	if d.srcUser != nil {
		dmsgLog.Logger.Errorf("DmsgService->subscribeUser: user isn't nil")
		return fmt.Errorf("DmsgService->subscribeUser: user isn't nil")
	}

	if d.destUserList[pubkey] != nil {
		dmsgLog.Logger.Errorf("DmsgService->subscribeUser: pubkey is already exist in destUserList")
		return fmt.Errorf("DmsgService->subscribeUser: pubkey is already exist in destUserList")
	}

	target, err := dmsgUser.NewTarget(d.baseService.GetCtx(), pubkey, getSig)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->subscribeUser: NewUser error: %v", err)
		return err
	}

	err = target.InitPubsub(d.pubsub, pubkey)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->subscribeUser: InitPubsub error: %v", err)
		return err
	}

	d.srcUser = &dmsgUser.User{
		Target:        *target,
		ServicePeerID: "",
	}

	// go d.BaseService.DiscoverRendezvousPeers()
	go d.handlePubsubProtocol(&d.srcUser.Target)

	dmsgLog.Logger.Debugf("DmsgService->subscribeUser end")
	return nil
}

func (d *MsgService) unsubscribeSrcUser() error {
	dmsgLog.Logger.Debugf("DmsgService->unsubscribeUser begin")
	if d.srcUser == nil {
		dmsgLog.Logger.Errorf("DmsgService->unsubscribeUser: userPubkey is not exist in destUserInfoList")
		return fmt.Errorf("DmsgService->unsubscribeUser: userPubkey is not exist in destUserInfoList")
	}
	d.srcUser.Close()
	d.srcUser = nil
	dmsgLog.Logger.Debugf("DmsgService->unsubscribeUser end")
	return nil
}

// channel
func (d *MsgService) createChannelService(pubkey string) error {
	dmsgLog.Logger.Debugf("DmsgService->createChannelService begin:\n channel key: %s", pubkey)
	find := false

	hostId := d.baseService.GetHost().ID().String()
	servicePeerList, _ := d.baseService.GetAvailableServicePeerList(hostId)
	srcPubkey, err := d.GetUserPubkeyHex()
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->createChannelService: GetUserPubkeyHex error: %v", err)
		return err
	}
	peerID := d.baseService.GetHost().ID().String()
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
		case <-d.baseService.GetCtx().Done():
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

// channel
func (d *MsgService) isAvailablePubChannel(pubKey string) bool {
	pubChannelInfo := len(d.channelList)
	if pubChannelInfo >= d.GetConfig().MaxPubChannelPubsubCount {
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
