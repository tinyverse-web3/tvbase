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
	createChannelProtocol *dmsgProtocol.CreatePubsubSProtocol
	pubsubMsgProtocol     *dmsgProtocol.PubsubMsgProtocol
	proxyPubsubList       map[string]*dmsgUser.ProxyPubsub
	stopCleanRestResource chan bool
	onReceiveMsg          msg.OnReceiveMsg
	onSendMsgResponse     msg.OnReceiveMsg
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
	d.proxyPubsubList = make(map[string]*dmsgUser.ProxyPubsub)
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
	err = d.handlePubsubProtocol(&d.LightUser.Target)
	if err != nil {
		log.Errorf("ChannelService->Start: handlePubsubProtocol error: %v", err)
		return err
	}

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

	if d.proxyPubsubList[pubkey] != nil {
		log.Errorf("ChannelService->SubscribeChannel: pubkey is already exist in channelList")
		return fmt.Errorf("ChannelService->SubscribeChannel: pubkey is already exist in channelList")
	}

	target, err := dmsgUser.NewTarget(pubkey, nil)
	if err != nil {
		log.Errorf("ChannelService->SubscribeChannel: NewTarget error: %v", err)
		return err
	}

	err = target.InitPubsub(pubkey)
	if err != nil {
		log.Errorf("ChannelService->subscribeUser: InitPubsub error: %v", err)
		return err
	}

	channel := &dmsgUser.ProxyPubsub{
		DestTarget: dmsgUser.DestTarget{
			Target:              *target,
			LastReciveTimestamp: time.Now().UnixNano(),
		},
	}

	if !d.EnableService {
		err = d.createChannelService(channel.Key.PubkeyHex)
		if err != nil {
			target.Close()
			return err
		}
	}

	// go d.BaseService.DiscoverRendezvousPeers()
	d.handlePubsubProtocol(&channel.Target)
	if err != nil {
		log.Errorf("ChannelService->SubscribeChannel: handlePubsubProtocol error: %v", err)
		err := channel.Target.Close()
		if err != nil {
			log.Warnf("ChannelService->SubscribeChannel: Target.Close error: %v", err)
			return err
		}
		return err
	}

	d.proxyPubsubList[pubkey] = channel
	log.Debug("ChannelService->SubscribeChannel end")
	return nil
}

func (d *ChannelService) UnsubscribeChannel(pubkey string) error {
	log.Debugf("ChannelService->UnsubscribeChannel begin\npubKey: %s", pubkey)

	channel := d.proxyPubsubList[pubkey]
	if channel == nil {
		log.Errorf("ChannelService->UnsubscribeChannel: channel is nil")
		return fmt.Errorf("ChannelService->UnsubscribeChannel: channel is nil")
	}
	err := channel.Close()
	if err != nil {
		log.Warnf("ChannelService->UnsubscribeChannel: channel.Close error: %v", err)
	}
	delete(d.proxyPubsubList, pubkey)

	log.Debug("ChannelService->UnsubscribeChannel end")
	return nil
}

func (d *ChannelService) UnsubscribeChannelList() error {
	for pubkey := range d.proxyPubsubList {
		d.UnsubscribeChannel(pubkey)
	}
	return nil
}

// sdk-msg
func (d *ChannelService) SendMsg(destPubkey string, content []byte) (*pb.MsgReq, error) {
	log.Debugf("ChannelService->SendMsg begin:\ndestPubkey: %s", destPubkey)
	requestProtoData, _, err := d.pubsubMsgProtocol.Request(d.LightUser.Key.PubkeyHex, destPubkey, content)
	if err != nil {
		log.Errorf("ChannelService->SendMsg: sendMsgProtocol.Request error: %v", err)
		return nil, err
	}
	request, ok := requestProtoData.(*pb.MsgReq)
	if !ok {
		log.Errorf("ChannelService->SendMsg: requestProtoData is not MsgReq")
		return nil, fmt.Errorf("ChannelService->SendMsg: requestProtoData is not MsgReq")
	}
	log.Debugf("ChannelService->SendMsg end")
	return request, nil
}

func (d *ChannelService) SetOnReceiveMsg(onReceiveMsg msg.OnReceiveMsg) {
	d.onReceiveMsg = onReceiveMsg
}

func (d *ChannelService) SetOnSendMsgResponse(onSendMsgResponse msg.OnReceiveMsg) {
	d.onSendMsgResponse = onSendMsgResponse
}

// DmsgServiceInterface
func (d *ChannelService) GetPublishTarget(pubkey string) (*dmsgUser.Target, error) {
	var target *dmsgUser.Target
	if d.proxyPubsubList[pubkey] != nil {
		target = &d.proxyPubsubList[pubkey].Target
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
func (d *ChannelService) OnCreatePubsubRequest(
	requestProtoData protoreflect.ProtoMessage) (any, any, bool, error) {
	log.Debugf("ChannelService->OnCreatePubusubRequest begin:\nrequestProtoData: %+v", requestProtoData)

	request, ok := requestProtoData.(*pb.CreatePubsubReq)
	if !ok {
		log.Errorf("ChannelService->OnCreatePubusubRequest: fail to convert requestProtoData to *pb.CreateMailboxReq")
		return nil, nil, false, fmt.Errorf("ChannelService->OnCreatePubusubRequest: cannot convert to *pb.CreateMailboxReq")
	}
	channelKey := request.ChannelKey
	isAvailable := d.isAvailableChannel(channelKey)
	if !isAvailable {
		return nil, nil, false, errors.New("ChannelService->OnCreatePubusubRequest: exceeded the maximum number of mailbox service")
	}
	channel := d.proxyPubsubList[channelKey]
	if channel != nil {
		log.Debugf("ChannelService->OnCreatePubusubRequest: channel already exist")
		retCode := &pb.RetCode{
			Code:   dmsgProtocol.AlreadyExistCode,
			Result: "ChannelService->OnCreatePubusubRequest: channel already exist",
		}
		return nil, retCode, false, nil
	}

	err := d.SubscribeChannel(channelKey)
	if err != nil {
		return nil, nil, false, err
	}

	log.Debugf("ChannelService->OnCreatePubusubRequest end")
	return nil, nil, false, nil
}

func (d *ChannelService) OnCreatePubsubResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	log.Debugf(
		"ChannelService->OnCreatePubsubResponse begin:\nrequestProtoData: %+v\nresponseProtoData: %+v",
		requestProtoData, responseProtoData)

	log.Debugf("ChannelService->OnCreatePubsubResponse end")
	return nil, nil
}

// MsgPpCallback
func (d *ChannelService) OnPubsubMsgRequest(
	requestProtoData protoreflect.ProtoMessage) (any, any, bool, error) {
	log.Debugf("ChannelService->OnPubsubMsgRequest begin:\nrequestProtoData: %+v", requestProtoData)
	request, ok := requestProtoData.(*pb.MsgReq)
	if !ok {
		log.Errorf("ChannelService->OnPubsubMsgRequest: fail to convert requestProtoData to *pb.MsgReq")
		return nil, nil, true, fmt.Errorf("ChannelService->OnPubsubMsgRequest: fail to convert requestProtoData to *pb.MsgReq")
	}

	destPubkey := request.DestPubkey
	if d.proxyPubsubList[destPubkey] == nil || d.LightUser.Key.PubkeyHex != destPubkey {
		log.Debugf("ChannelService->OnPubsubMsgRequest: user/channel pubkey is not exist")
		return nil, nil, true, fmt.Errorf("ChannelService->OnPubsubMsgRequest: user/channel pubkey is not exist")
	}

	if d.proxyPubsubList[destPubkey] != nil {
		if d.onReceiveMsg != nil {
			srcPubkey := request.BasicData.Pubkey
			destPubkey := request.DestPubkey
			msgDirection := msg.MsgDirection.From
			responseContent, err := d.onReceiveMsg(
				srcPubkey,
				destPubkey,
				request.Content,
				request.BasicData.TS,
				request.BasicData.ID,
				msgDirection)
			var retCode *pb.RetCode
			if err != nil {
				retCode = &pb.RetCode{
					Code:   dmsgProtocol.AlreadyExistCode,
					Result: "ChannelService->OnPubsubMsgRequest: " + err.Error(),
				}
			}
			return responseContent, retCode, false, nil
		} else {
			log.Errorf("ChannelService->OnPubsubMsgRequest: OnReceiveMsg is nil")
		}
		return nil, nil, false, nil
	} else if d.LightUser.Key.PubkeyHex == destPubkey {
		return nil, nil, false, nil
	}
	log.Debugf("ChannelService->OnPubsubMsgRequest end")
	return nil, nil, false, nil
}

func (d *ChannelService) OnPubsubMsgResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	log.Debugf(
		"ChannelService->OnPubsubMsgResponse begin:\nrequestProtoData: %+v\nresponseProtoData: %+v",
		requestProtoData, responseProtoData)

	request, ok := requestProtoData.(*pb.MsgReq)
	if !ok {
		log.Errorf("ChannelService->OnPubsubMsgResponse: fail to convert requestProtoData to *pb.MsgReq")
		return nil, fmt.Errorf("ChannelService->OnPubsubMsgResponse: fail to convert requestProtoData to *pb.MsgReq")
	}

	response, ok := responseProtoData.(*pb.MsgRes)
	if !ok {
		log.Errorf("ChannelService->OnPubsubMsgResponse: fail to convert responseProtoData to *pb.MsgRes")
		return nil, fmt.Errorf("ChannelService->OnPubsubMsgResponse: fail to convert responseProtoData to *pb.MsgRes")
	}

	if response.RetCode.Code != 0 {
		log.Warnf("ChannelService->OnPubsubMsgResponse: fail RetCode: %+v", response.RetCode)
		return nil, fmt.Errorf("ChannelService->OnPubsubMsgResponse: fail RetCode: %+v", response.RetCode)
	} else {
		if d.onSendMsgResponse != nil {
			srcPubkey := request.BasicData.Pubkey
			destPubkey := request.DestPubkey
			msgDirection := msg.MsgDirection.From
			d.onSendMsgResponse(
				srcPubkey,
				destPubkey,
				request.Content,
				request.BasicData.TS,
				request.BasicData.ID,
				msgDirection)
		} else {
			log.Warnf("ChannelService->OnPubsubMsgRequest: onSendMsgResponse is nil")
		}
	}
	log.Debugf("ChannelService->OnPubsubMsgResponse end")
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
				for pubChannelPubkey, pubsub := range d.proxyPubsubList {
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

func (d *ChannelService) handlePubsubProtocol(target *dmsgUser.Target) error {
	ctx := d.TvBase.GetCtx()
	protocolDataChan, err := dmsgServiceCommon.WaitMessage(ctx, target.Key.PubkeyHex)
	if err != nil {
		return err
	}
	log.Debugf("ChannelService->handlePubsubProtocol: protocolDataChan: %+v", protocolDataChan)

	go func() {
		for {
			select {
			case protocolHandle, ok := <-protocolDataChan:
				if !ok {
					return
				}
				pid := protocolHandle.PID
				log.Debugf("ChannelService->handlePubsubProtocol: \npid: %d\ntopicName: %s", pid, target.Pubsub.Topic.String())

				handle := d.ProtocolHandleList[pid]
				if handle == nil {
					log.Warnf("ChannelService->handlePubsubProtocol: no handle for pid: %d", pid)
					continue
				}
				msgRequestPID := d.pubsubMsgProtocol.Adapter.GetRequestPID()
				msgResponsePID := d.pubsubMsgProtocol.Adapter.GetResponsePID()
				data := protocolHandle.Data
				switch pid {
				case msgRequestPID:
					err = handle.HandleRequestData(data)
					if err != nil {
						log.Warnf("ChannelService->handlePubsubProtocol: HandleRequestData error: %v", err)
					}
					continue
				case msgResponsePID:
					continue
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return nil
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
		_, createPubsubDoneChan, err := d.createChannelProtocol.Request(servicePeerID, srcPubkey, pubkey)
		if err != nil {
			continue
		}

		select {
		case responseProtoData := <-createPubsubDoneChan:
			log.Debugf("ChannelService->createChannelService:\ncreateChannelResponseProtoData: %+v",
				responseProtoData)
			response, ok := responseProtoData.(*pb.CreatePubsubRes)
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
func (d *ChannelService) isAvailableChannel(pubKey string) bool {
	pubChannelInfo := len(d.proxyPubsubList)
	if pubChannelInfo >= d.GetConfig().MaxChannelCount {
		log.Warnf("ChannelService->isAvailableMailbox: exceeded the maximum number of mailbox services, current destUserCount:%v", pubChannelInfo)
		return false
	}
	return true
}
