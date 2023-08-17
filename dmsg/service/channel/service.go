package channel

import (
	"errors"
	"fmt"
	"time"

	ipfsLog "github.com/ipfs/go-log/v2"
	tvbaseCommon "github.com/tinyverse-web3/tvbase/common"
	dmsgKey "github.com/tinyverse-web3/tvbase/dmsg/common/key"

	"github.com/tinyverse-web3/tvbase/dmsg/common/msg"
	dmsgUser "github.com/tinyverse-web3/tvbase/dmsg/common/user"
	dmsgCommonUtil "github.com/tinyverse-web3/tvbase/dmsg/common/util"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/adapter"
	dmsgServiceCommon "github.com/tinyverse-web3/tvbase/dmsg/service/common"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var log = ipfsLog.Logger("dmsg.service.channel")

type ChannelService struct {
	dmsgServiceCommon.LightUserService
	createChannelProtocol *dmsgProtocol.ChannelSProtocol
	pubsubMsgProtocol     *dmsgProtocol.PubsubMsgProtocol
	channelList           map[string]*dmsgUser.Channel
	stopCleanRestResource chan bool
	onReceiveMsg          msg.OnReceiveMsg
}

func CreateService(tvbase tvbaseCommon.TvBaseService) (*ChannelService, error) {
	d := &ChannelService{}
	err := d.Init(tvbase)
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (d *ChannelService) Init(tvbase tvbaseCommon.TvBaseService) error {
	err := d.LightUserService.Init(tvbase)
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
	log.Debug("ChannelService->Start begin")
	err := d.LightUserService.Start(
		enableService, pubkeyData, getSig, true)
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
	d.pubsubMsgProtocol = adapter.NewPubsubMsgProtocol(ctx, host, d, d)
	d.RegistPubsubProtocol(d.pubsubMsgProtocol.Adapter.GetRequestPID(), d.pubsubMsgProtocol)
	d.RegistPubsubProtocol(d.pubsubMsgProtocol.Adapter.GetResponsePID(), d.pubsubMsgProtocol)
	// user
	go d.handlePubsubProtocol(&d.LightUser.Target)

	log.Debug("ChannelService->Start end")
	return nil
}

func (d *ChannelService) Stop() error {
	log.Debug("ChannelService->Stop begin")
	err := d.LightUserService.Stop()
	if err != nil {
		return err
	}

	d.UnsubscribeChannelList()
	select {
	case d.stopCleanRestResource <- true:
		log.Debugf("ChannelService->Stop: succ send stopCleanRestResource")
	default:
		log.Debugf("ChannelService->Stop: no receiver for stopCleanRestResource")
	}
	close(d.stopCleanRestResource)
	log.Debug("ChannelService->Stop end")
	return nil
}

// sdk-channel
func (d *ChannelService) SubscribeChannel(pubkey string) error {
	log.Debugf("ChannelService->SubscribeChannel begin:\npubkey: %s", pubkey)

	if d.channelList[pubkey] != nil {
		log.Errorf("ChannelService->SubscribeChannel: pubkey is already exist in channelList")
		return fmt.Errorf("ChannelService->SubscribeChannel: pubkey is already exist in channelList")
	}

	target, err := dmsgUser.NewTarget(d.TvBase.GetCtx(), pubkey, nil)
	if err != nil {
		log.Errorf("ChannelService->SubscribeChannel: NewTarget error: %v", err)
		return err
	}

	err = target.InitPubsub(pubkey)
	if err != nil {
		log.Errorf("ChannelService->subscribeUser: InitPubsub error: %v", err)
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

	log.Debug("ChannelService->SubscribeChannel end")
	return nil
}

func (d *ChannelService) UnsubscribeChannel(pubkey string) error {
	log.Debugf("ChannelService->UnsubscribeChannel begin\npubKey: %s", pubkey)

	channel := d.channelList[pubkey]
	if channel == nil {
		log.Errorf("ChannelService->UnsubscribeChannel: channel is nil")
		return fmt.Errorf("ChannelService->UnsubscribeChannel: channel is nil")
	}
	err := channel.Close()
	if err != nil {
		log.Warnf("ChannelService->UnsubscribeChannel: channel.Close error: %v", err)
	}
	delete(d.channelList, pubkey)

	log.Debug("ChannelService->UnsubscribeChannel end")
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
	log.Debugf("ChannelService->SendMsg begin:\ndestPubkey: %s", destPubkey)
	requestProtoData, _, err := d.pubsubMsgProtocol.Request(d.LightUser.Key.PubkeyHex, destPubkey, content)
	if err != nil {
		log.Errorf("ChannelService->SendMsg: sendMsgProtocol.Request error: %v", err)
		return nil, err
	}
	request, ok := requestProtoData.(*pb.SendMsgReq)
	if !ok {
		log.Errorf("ChannelService->SendMsg: requestProtoData is not SendMsgReq")
		return nil, fmt.Errorf("ChannelService->SendMsg: requestProtoData is not SendMsgReq")
	}
	log.Debugf("ChannelService->SendMsg end")
	return request, nil
}

func (d *ChannelService) SetOnReceiveMsg(onReceiveMsg msg.OnReceiveMsg) {
	d.onReceiveMsg = onReceiveMsg
}

// DmsgServiceInterface
func (d *ChannelService) GetPublishTarget(pubkey string) (*dmsgUser.Target, error) {
	var target *dmsgUser.Target
	if d.channelList[pubkey] != nil {
		target = &d.channelList[pubkey].Target
	} else if d.LightUser.Key.PubkeyHex == pubkey {
		target = &d.LightUser.Target
	}

	if target == nil {
		log.Errorf("ChannelService->GetPublishTarget: target is nil")
		return nil, fmt.Errorf("ChannelService->GetPublishTarget: target is nil")
	}
	return target, nil
}

// MsgSpCallback
func (d *ChannelService) OnCreateChannelRequest(
	requestProtoData protoreflect.ProtoMessage) (any, any, bool, error) {
	log.Debugf("dmsgService->OnCreateChannelRequest begin:\nrequestProtoData: %+v", requestProtoData)

	request, ok := requestProtoData.(*pb.CreateChannelReq)
	if !ok {
		log.Errorf("dmsgService->OnCreateChannelRequest: fail to convert requestProtoData to *pb.CreateMailboxReq")
		return nil, nil, false, fmt.Errorf("dmsgService->OnCreateChannelRequest: cannot convert to *pb.CreateMailboxReq")
	}
	channelKey := request.ChannelKey
	isAvailable := d.isAvailablePubChannel(channelKey)
	if !isAvailable {
		return nil, nil, false, errors.New("dmsgService->OnCreateChannelRequest: exceeded the maximum number of mailbox service")
	}
	channel := d.channelList[channelKey]
	if channel != nil {
		log.Debugf("dmsgService->OnCreateChannelRequest: channel already exist")
		retCode := &pb.RetCode{
			Code:   dmsgProtocol.AlreadyExistCode,
			Result: "dmsgService->OnCreateChannelRequest: channel already exist",
		}
		return nil, retCode, false, nil
	}

	err := d.SubscribeChannel(channelKey)
	if err != nil {
		return nil, nil, false, err
	}

	log.Debugf("dmsgService->OnCreateChannelRequest end")
	return nil, nil, false, nil
}

func (d *ChannelService) OnCreateChannelResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	log.Debugf(
		"dmsgService->OnCreateChannelResponse begin:\nrequestProtoData: %+v\nresponseProtoData: %+v",
		requestProtoData, responseProtoData)

	log.Debugf("dmsgService->OnCreateChannelResponse end")
	return nil, nil
}

// MsgPpCallback
func (d *ChannelService) OnPubsubMsgRequest(
	requestProtoData protoreflect.ProtoMessage) (any, any, bool, error) {
	log.Debugf("dmsgService->OnPubsubMsgRequest begin:\nrequestProtoData: %+v", requestProtoData)
	request, ok := requestProtoData.(*pb.SendMsgReq)
	if !ok {
		log.Errorf("ChannelService->OnPubsubMsgRequest: fail to convert requestProtoData to *pb.SendMsgReq")
		return nil, nil, true, fmt.Errorf("ChannelService->OnPubsubMsgRequest: fail to convert requestProtoData to *pb.SendMsgReq")
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
		log.Errorf("ChannelService->OnPubsubMsgRequest: OnReceiveMsg is nil")
	}

	log.Debugf("dmsgService->OnPubsubMsgRequest end")
	return nil, nil, true, nil
}

func (d *ChannelService) OnPubsubMsgResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	log.Debugf(
		"dmsgService->OnPubsubMsgResponse begin:\nrequestProtoData: %+v\nresponseProtoData: %+v",
		requestProtoData, responseProtoData)
	// never here
	log.Debugf("dmsgService->OnPubsubMsgResponse end")
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
		protocolID, protocolData, protocolHandle, err := d.WaitPubsubProtocolData(target)
		if err != nil {
			log.Warnf("ChannelService->handlePubsubProtocol: target.WaitMsg error: %+v", err)
			return
		}

		if protocolHandle == nil {
			continue
		}

		msgRequestPID := d.pubsubMsgProtocol.Adapter.GetRequestPID()
		msgResponsePID := d.pubsubMsgProtocol.Adapter.GetResponsePID()
		log.Debugf("MailboxService->handlePubsubProtocol: protocolID: %d", protocolID)

		switch protocolID {
		case msgRequestPID:
			err = protocolHandle.HandleRequestData(protocolData)
			if err != nil {
				log.Warnf("ChannelService->handlePubsubProtocol: HandleRequestData error: %v", err)
			}
			continue
		case msgResponsePID:
			continue
		}
	}
}

// channel
func (d *ChannelService) createChannelService(pubkey string) error {
	log.Debugf("ChannelService->createChannelService begin:\n channel key: %s", pubkey)
	find := false

	hostId := d.TvBase.GetHost().ID().String()
	servicePeerList, _ := d.TvBase.GetAvailableServicePeerList(hostId)
	srcPubkey, err := d.GetUserPubkeyHex()
	if err != nil {
		log.Errorf("ChannelService->createChannelService: GetUserPubkeyHex error: %v", err)
		return err
	}
	peerID := d.TvBase.GetHost().ID().String()
	for _, servicePeerID := range servicePeerList {
		log.Debugf("ChannelService->createChannelService: servicePeerID: %s", servicePeerID)
		if peerID == servicePeerID.String() {
			continue
		}
		_, createChannelDoneChan, err := d.createChannelProtocol.Request(servicePeerID, srcPubkey, pubkey)
		if err != nil {
			continue
		}

		select {
		case createChannelResponseProtoData := <-createChannelDoneChan:
			log.Debugf("ChannelService->createChannelService:\ncreateChannelResponseProtoData: %+v",
				createChannelResponseProtoData)
			response, ok := createChannelResponseProtoData.(*pb.CreateChannelRes)
			if !ok || response == nil {
				log.Errorf("ChannelService->createChannelService: createPubChannelResponseProtoData is not CreatePubChannelRes")
				continue
			}
			if response.RetCode.Code < 0 {
				log.Errorf("ChannelService->createChannelService: createPubChannel fail")
				continue
			} else {
				log.Debugf("ChannelService->createChannelService: createPubChannel success")
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
		log.Error("ChannelService->createChannelService: no available service peer")
		return fmt.Errorf("ChannelService->createChannelService: no available service peer")
	}
	log.Debug("ChannelService->createChannelService end")
	return nil
}

// channel
func (d *ChannelService) isAvailablePubChannel(pubKey string) bool {
	pubChannelInfo := len(d.channelList)
	if pubChannelInfo >= d.GetConfig().MaxChannelCount {
		log.Warnf("dmsgService->isAvailableMailbox: exceeded the maximum number of mailbox services, current destUserCount:%v", pubChannelInfo)
		return false
	}
	return true
}
