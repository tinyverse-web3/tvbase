package channel

import (
	"errors"
	"fmt"
	"time"

	tvbaseCommon "github.com/tinyverse-web3/tvbase/common"
	dmsgKey "github.com/tinyverse-web3/tvbase/dmsg/common/key"
	"github.com/tinyverse-web3/tvbase/dmsg/common/log"
	"github.com/tinyverse-web3/tvbase/dmsg/common/msg"
	dmsgUser "github.com/tinyverse-web3/tvbase/dmsg/common/user"
	dmsgCommonUtil "github.com/tinyverse-web3/tvbase/dmsg/common/util"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/adapter"
	dmsgCommonService "github.com/tinyverse-web3/tvbase/dmsg/service/common"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type ChannelService struct {
	dmsgCommonService.LightUserService
	createChannelProtocol *dmsgProtocol.ChannelSProtocol
	sendMsgProtocol       *dmsgProtocol.MsgPProtocol
	channelList           map[string]*dmsgUser.Channel
	stopCleanRestResource chan bool
	onReceiveMsg          msg.OnReceiveMsg
}

func CreateService(tvbaseService tvbaseCommon.TvBaseService) (*ChannelService, error) {
	d := &ChannelService{}
	err := d.Init(tvbaseService)
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (d *ChannelService) Init(tvbaseService tvbaseCommon.TvBaseService) error {
	err := d.LightUserService.Init(tvbaseService)
	if err != nil {
		return err
	}
	d.channelList = make(map[string]*dmsgUser.Channel)
	return nil
}

// sdk-common
func (d *ChannelService) Start(
	enableService bool,
	pubkeyData []byte,
	getSig dmsgKey.GetSigCallback) error {
	log.Logger.Debug("ChannelService->Start begin")
	err := d.LightUserService.Start(enableService, pubkeyData, getSig, true)
	if err != nil {
		return err
	}
	if d.EnableService {
		d.stopCleanRestResource = make(chan bool)
		d.cleanRestResource()
	}

	ctx := d.TvBase.GetCtx()
	host := d.TvBase.GetHost()
	// stream protocol
	d.createChannelProtocol = adapter.NewCreateChannelProtocol(ctx, host, d, d)

	// pubsub protocol
	d.sendMsgProtocol = adapter.NewSendMsgProtocol(ctx, host, d, d)
	d.RegistPubsubProtocol(d.sendMsgProtocol.Adapter.GetRequestPID(), d.sendMsgProtocol)

	// user
	go d.handlePubsubProtocol(&d.LightUser.Target)

	log.Logger.Debug("ChannelService->Start end")
	return nil
}

func (d *ChannelService) Stop() error {
	log.Logger.Debug("ChannelService->Stop begin")
	err := d.LightUserService.Stop()
	if err != nil {
		return err
	}

	d.UnsubscribeChannelList()
	d.stopCleanRestResource <- true
	close(d.stopCleanRestResource)
	log.Logger.Debug("ChannelService->Stop end")
	return nil
}

// sdk-channel
func (d *ChannelService) SubscribeChannel(pubkey string) error {
	log.Logger.Debugf("ChannelService->SubscribeChannel begin:\npubkey: %s", pubkey)

	if d.channelList[pubkey] != nil {
		log.Logger.Errorf("ChannelService->SubscribeChannel: pubkey is already exist in channelList")
		return fmt.Errorf("ChannelService->SubscribeChannel: pubkey is already exist in channelList")
	}

	target, err := dmsgUser.NewTarget(d.TvBase.GetCtx(), pubkey, nil)
	if err != nil {
		log.Logger.Errorf("ChannelService->SubscribeChannel: NewTarget error: %v", err)
		return err
	}

	err = target.InitPubsub(d.Pubsub, pubkey)
	if err != nil {
		log.Logger.Errorf("ChannelService->subscribeUser: InitPubsub error: %v", err)
		return err
	}

	channel := &dmsgUser.Channel{
		DestTarget: dmsgUser.DestTarget{
			Target:              *target,
			LastReciveTimestamp: time.Now().UnixNano(),
		},
	}
	d.channelList[pubkey] = channel

	if !d.EnableService {
		err = d.createChannelService(channel.Key.PubkeyHex)
		if err != nil {
			target.Close()
			delete(d.channelList, pubkey)
			return err
		}
	}

	// go d.BaseService.DiscoverRendezvousPeers()
	go d.handlePubsubProtocol(&channel.Target)

	log.Logger.Debug("ChannelService->SubscribeChannel end")
	return nil
}

func (d *ChannelService) UnsubscribeChannel(pubkey string) error {
	log.Logger.Debugf("ChannelService->UnsubscribeChannel begin\npubKey: %s", pubkey)

	channel := d.channelList[pubkey]
	if channel == nil {
		log.Logger.Errorf("ChannelService->UnsubscribeChannel: channel is nil")
		return fmt.Errorf("ChannelService->UnsubscribeChannel: channel is nil")
	}
	err := channel.Close()
	if err != nil {
		log.Logger.Warnf("ChannelService->UnsubscribeChannel: channel.Close error: %v", err)
	}
	delete(d.channelList, pubkey)

	log.Logger.Debug("ChannelService->UnsubscribeChannel end")
	return nil
}

func (d *ChannelService) UnsubscribeChannelList() error {
	for pubkey := range d.channelList {
		d.UnsubscribeChannel(pubkey)
	}
	return nil
}

// sdk-msg
func (d *ChannelService) SendMsg(destPubkey string, content []byte) (*pb.SendMsgReq, error) {
	log.Logger.Debugf("ChannelService->SendMsg begin:\ndestPubkey: %s", destPubkey)
	requestProtoData, _, err := d.sendMsgProtocol.Request(d.LightUser.Key.PubkeyHex, destPubkey, content)
	if err != nil {
		log.Logger.Errorf("ChannelService->SendMsg: sendMsgProtocol.Request error: %v", err)
		return nil, err
	}
	request, ok := requestProtoData.(*pb.SendMsgReq)
	if !ok {
		log.Logger.Errorf("ChannelService->SendMsg: requestProtoData is not SendMsgReq")
		return nil, fmt.Errorf("ChannelService->SendMsg: requestProtoData is not SendMsgReq")
	}
	log.Logger.Debugf("ChannelService->SendMsg end")
	return request, nil
}

func (d *ChannelService) SetOnReceiveMsg(onReceiveMsg msg.OnReceiveMsg) {
	d.onReceiveMsg = onReceiveMsg
}

// DmsgServiceInterface
func (d *ChannelService) GetPublishTarget(pubkey string) (*dmsgUser.Target, error) {
	var target *dmsgUser.Target = nil
	user := d.channelList[pubkey]
	if user == nil {
		if d.LightUser.Key.PubkeyHex != pubkey {
			log.Logger.Errorf("ChannelService->GetPublishTarget: pubkey not exist")
			return nil, fmt.Errorf("ChannelService->GetPublishTarget: pubkey not exist")
		} else {
			target = &d.LightUser.Target
		}
	} else {
		target = &user.Target
	}
	return target, nil
}

// MsgSpCallback
func (d *ChannelService) OnCreateChannelRequest(
	requestProtoData protoreflect.ProtoMessage) (any, any, error) {
	log.Logger.Debugf("dmsgService->OnCreateChannelRequest begin:\nrequestProtoData: %+v", requestProtoData)

	request, ok := requestProtoData.(*pb.CreateChannelReq)
	if !ok {
		log.Logger.Errorf("dmsgService->OnCreateChannelRequest: fail to convert requestProtoData to *pb.CreateMailboxReq")
		return nil, nil, fmt.Errorf("dmsgService->OnCreateChannelRequest: cannot convert to *pb.CreateMailboxReq")
	}
	channelKey := request.ChannelKey
	isAvailable := d.isAvailablePubChannel(channelKey)
	if !isAvailable {
		return nil, nil, errors.New("dmsgService->OnCreateChannelRequest: exceeded the maximum number of mailbox service")
	}
	channel := d.channelList[channelKey]
	if channel != nil {
		log.Logger.Errorf("dmsgService->OnCreateChannelRequest: channel already exist")
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

	log.Logger.Debugf("dmsgService->OnCreateChannelRequest end")
	return nil, nil, nil
}

func (d *ChannelService) OnCreateChannelResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	log.Logger.Debugf(
		"dmsgService->OnCreateChannelResponse begin:\nrequestProtoData: %+v\nresponseProtoData: %+v",
		requestProtoData, responseProtoData)

	log.Logger.Debugf("dmsgService->OnCreateChannelResponse end")
	return nil, nil
}

// MsgPpCallback
func (d *ChannelService) OnSendMsgRequest(
	requestProtoData protoreflect.ProtoMessage) (any, any, error) {
	log.Logger.Debugf("dmsgService->OnSendMsgRequest begin:\nrequestProtoData: %+v", requestProtoData)
	request, ok := requestProtoData.(*pb.SendMsgReq)
	if !ok {
		log.Logger.Errorf("ChannelService->OnSendMsgRequest: fail to convert requestProtoData to *pb.SendMsgReq")
		return nil, nil, fmt.Errorf("ChannelService->OnSendMsgRequest: fail to convert requestProtoData to *pb.SendMsgReq")
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
		log.Logger.Errorf("ChannelService->OnSendMsgRequest: OnReceiveMsg is nil")
	}

	log.Logger.Debugf("dmsgService->OnSendMsgRequest end")
	return nil, nil, nil
}

func (d *ChannelService) OnSendMsgResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	log.Logger.Debugf(
		"dmsgService->OnSendMsgResponse begin:\nrequestProtoData: %+v\nresponseProtoData: %+v",
		requestProtoData, responseProtoData)
	response, ok := responseProtoData.(*pb.SendMsgRes)
	if !ok {
		log.Logger.Errorf("ChannelService->OnSendMsgResponse: cannot convert responseProtoData to *pb.SendMsgRes")
		return nil, fmt.Errorf("ChannelService->OnSendMsgResponse: cannot convert responseProtoData to *pb.SendMsgRes")
	}
	if response.RetCode.Code != 0 {
		log.Logger.Warnf("ChannelService->OnSendMsgResponse: fail RetCode: %+v", response.RetCode)
		return nil, fmt.Errorf("ChannelService->OnSendMsgResponse: fail RetCode: %+v", response.RetCode)
	}
	log.Logger.Debugf("dmsgService->OnSendMsgResponse end")
	return nil, nil
}

// common
func (d *ChannelService) cleanRestResource() {
	go func() {
		ticker := time.NewTicker(3 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-d.stopCleanRestResource:
				return
			case <-ticker.C:
				for pubChannelPubkey, pubsub := range d.channelList {
					days := dmsgCommonUtil.DaysBetween(pubsub.LastReciveTimestamp, time.Now().UnixNano())
					if days >= d.GetConfig().KeepPubChannelDay {
						d.UnsubscribeChannel(pubChannelPubkey)
						return
					}
				}

				continue
			case <-d.TvBase.GetCtx().Done():
				return
			}
		}
	}()
}

func (d *ChannelService) handlePubsubProtocol(target *dmsgUser.Target) {
	for {
		m, err := target.WaitMsg()
		if err != nil {
			log.Logger.Warnf("dmsgService->handlePubsubProtocol: target.WaitMsg error: %+v", err)
			return
		}

		log.Logger.Debugf("dmsgService->handlePubsubProtocol:\ntopic: %s\nreceivedFrom: %+v", m.Topic, m.ReceivedFrom)

		protocolID, protocolIDLen, err := d.CheckProtocolData(m.Data)
		if err != nil {
			log.Logger.Errorf("dmsgService->handlePubsubProtocol: CheckPubsubData error: %v", err)
			continue
		}
		protocolData := m.Data[protocolIDLen:]
		protocolHandle := d.ProtocolHandleList[protocolID]
		if protocolHandle != nil {
			err = protocolHandle.HandleRequestData(protocolData)
			if err != nil {
				log.Logger.Warnf("dmsgService->handlePubsubProtocol: HandleRequestData error: %v", err)
			}
			continue
		} else {
			log.Logger.Warnf("dmsgService->handlePubsubProtocol: no protocolHandle for protocolID: %d", protocolID)
		}
	}
}

// channel
func (d *ChannelService) createChannelService(pubkey string) error {
	log.Logger.Debugf("ChannelService->createChannelService begin:\n channel key: %s", pubkey)
	find := false

	hostId := d.TvBase.GetHost().ID().String()
	servicePeerList, _ := d.TvBase.GetAvailableServicePeerList(hostId)
	srcPubkey, err := d.GetUserPubkeyHex()
	if err != nil {
		log.Logger.Errorf("ChannelService->createChannelService: GetUserPubkeyHex error: %v", err)
		return err
	}
	peerID := d.TvBase.GetHost().ID().String()
	for _, servicePeerID := range servicePeerList {
		log.Logger.Debugf("ChannelService->createChannelService: servicePeerID: %s", servicePeerID)
		if peerID == servicePeerID.String() {
			continue
		}
		_, createChannelDoneChan, err := d.createChannelProtocol.Request(servicePeerID, srcPubkey, pubkey)
		if err != nil {
			continue
		}

		select {
		case createChannelResponseProtoData := <-createChannelDoneChan:
			log.Logger.Debugf("ChannelService->createChannelService:\ncreateChannelResponseProtoData: %+v",
				createChannelResponseProtoData)
			response, ok := createChannelResponseProtoData.(*pb.CreateChannelRes)
			if !ok || response == nil {
				log.Logger.Errorf("ChannelService->createChannelService: createPubChannelResponseProtoData is not CreatePubChannelRes")
				continue
			}
			if response.RetCode.Code < 0 {
				log.Logger.Errorf("ChannelService->createChannelService: createPubChannel fail")
				continue
			} else {
				log.Logger.Debugf("ChannelService->createChannelService: createPubChannel success")
				find = true
				return nil
			}

		case <-time.After(time.Second * 3):
			continue
		case <-d.TvBase.GetCtx().Done():
			return fmt.Errorf("ChannelService->createChannelService: BaseService.GetCtx().Done()")
		}
	}
	if !find {
		log.Logger.Error("ChannelService->createChannelService: no available service peer")
		return fmt.Errorf("ChannelService->createChannelService: no available service peer")
	}
	log.Logger.Debug("ChannelService->createChannelService end")
	return nil
}

// channel
func (d *ChannelService) isAvailablePubChannel(pubKey string) bool {
	pubChannelInfo := len(d.channelList)
	if pubChannelInfo >= d.GetConfig().MaxPubChannelPubsubCount {
		log.Logger.Warnf("dmsgService->isAvailableMailbox: exceeded the maximum number of mailbox services, current destUserCount:%v", pubChannelInfo)
		return false
	}
	return true
}
