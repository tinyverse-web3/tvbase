package common

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
	"google.golang.org/protobuf/reflect/protoreflect"
)

var log = ipfsLog.Logger("dmsg.service.proxypubsub")

type ProxyPubsubService struct {
	LightUserService
	createPubsubProtocol  *dmsgProtocol.CreatePubsubSProtocol
	pubsubMsgProtocol     *dmsgProtocol.PubsubMsgProtocol
	proxyPubsubList       map[string]*dmsgUser.ProxyPubsub
	stopCleanRestResource chan bool
	onReceiveMsg          msg.OnReceiveMsg
	onSendMsgResponse     msg.OnReceiveMsg
}

func (d *ProxyPubsubService) Init(tvbase tvbaseCommon.TvBaseService) error {
	err := d.LightUserService.Init(tvbase)
	if err != nil {
		return err
	}
	d.proxyPubsubList = make(map[string]*dmsgUser.ProxyPubsub)
	return nil
}

// sdk-common
func (d *ProxyPubsubService) Start(
	enableService bool,
	pubkeyData []byte,
	getSig dmsgKey.GetSigCallback,
	createPubsubProtocol *dmsgProtocol.CreatePubsubSProtocol,
) error {
	log.Debug("ProxyPubsubService->Start begin")
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
	d.createPubsubProtocol = createPubsubProtocol

	// pubsub protocol
	d.pubsubMsgProtocol = adapter.NewPubsubMsgProtocol(ctx, host, d, d)
	d.RegistPubsubProtocol(d.pubsubMsgProtocol.Adapter.GetRequestPID(), d.pubsubMsgProtocol)
	d.RegistPubsubProtocol(d.pubsubMsgProtocol.Adapter.GetResponsePID(), d.pubsubMsgProtocol)

	// user
	err = d.handlePubsubProtocol(&d.LightUser.Target)
	if err != nil {
		log.Errorf("ProxyPubsubService->Start: handlePubsubProtocol error: %v", err)
		return err
	}

	log.Debug("ProxyPubsubService->Start end")
	return nil
}

func (d *ProxyPubsubService) Stop() error {
	log.Debug("ProxyPubsubService->Stop begin")
	err := d.LightUserService.Stop()
	if err != nil {
		return err
	}

	d.UnsubscribePubsubList()
	select {
	case d.stopCleanRestResource <- true:
		log.Debugf("ProxyPubsubService->Stop: succ send stopCleanRestResource")
	default:
		log.Debugf("ProxyPubsubService->Stop: no receiver for stopCleanRestResource")
	}
	close(d.stopCleanRestResource)
	log.Debug("ProxyPubsubService->Stop end")
	return nil
}

func (d *ProxyPubsubService) GetProxyPubsub(pubkey string) *dmsgUser.ProxyPubsub {
	return d.proxyPubsubList[pubkey]
}

func (d *ProxyPubsubService) SubscribePubsub(pubkey string, createPubsubProxy bool) error {
	log.Debugf("ProxyPubsubService->SubscribeChannel begin:\npubkey: %s", pubkey)

	if d.proxyPubsubList[pubkey] != nil {
		log.Errorf("ProxyPubsubService->SubscribeChannel: pubkey is already exist in channelList")
		return fmt.Errorf("ProxyPubsubService->SubscribeChannel: pubkey is already exist in channelList")
	}

	target, err := dmsgUser.NewTarget(pubkey, nil)
	if err != nil {
		log.Errorf("ProxyPubsubService->SubscribeChannel: NewTarget error: %v", err)
		return err
	}

	err = target.InitPubsub(pubkey)
	if err != nil {
		log.Errorf("ProxyPubsubService->subscribeUser: target.InitPubsub error: %v", err)
		return err
	}

	proxyPubsub := &dmsgUser.ProxyPubsub{
		DestTarget: dmsgUser.DestTarget{
			Target:              *target,
			LastReciveTimestamp: time.Now().UnixNano(),
		},
	}

	if createPubsubProxy {
		err = d.CreatePubsubService(proxyPubsub.Key.PubkeyHex)
		if err != nil {
			target.Close()
			return err
		}
	}

	d.handlePubsubProtocol(&proxyPubsub.Target)
	if err != nil {
		log.Errorf("ProxyPubsubService->SubscribeChannel: handlePubsubProtocol error: %v", err)
		err := proxyPubsub.Target.Close()
		if err != nil {
			log.Warnf("ProxyPubsubService->SubscribeChannel: Target.Close error: %v", err)
			return err
		}
		return err
	}

	d.proxyPubsubList[pubkey] = proxyPubsub
	log.Debug("ProxyPubsubService->SubscribeChannel end")
	return nil
}

func (d *ProxyPubsubService) UnsubscribePubsub(pubkey string) error {
	log.Debugf("ProxyPubsubService->UnsubscribePubsub begin\npubKey: %s", pubkey)

	proxyPubsub := d.proxyPubsubList[pubkey]
	if proxyPubsub == nil {
		log.Errorf("ProxyPubsubService->UnsubscribePubsub: proxyPubsub is nil")
		return fmt.Errorf("ProxyPubsubService->UnsubscribePubsub: proxyPubsub is nil")
	}
	err := proxyPubsub.Close()
	if err != nil {
		log.Warnf("ProxyPubsubService->UnsubscribePubsub: proxyPubsub.Close error: %v", err)
	}
	delete(d.proxyPubsubList, pubkey)

	log.Debug("ProxyPubsubService->UnsubscribePubsub end")
	return nil
}

func (d *ProxyPubsubService) UnsubscribePubsubList() error {
	for pubkey := range d.proxyPubsubList {
		d.UnsubscribePubsub(pubkey)
	}
	return nil
}

func (d *ProxyPubsubService) SendMsg(destPubkey string, content []byte) (*pb.MsgReq, error) {
	log.Debugf("ProxyPubsubService->SendMsg begin:\ndestPubkey: %s", destPubkey)
	requestProtoData, _, err := d.pubsubMsgProtocol.Request(d.LightUser.Key.PubkeyHex, destPubkey, content)
	if err != nil {
		log.Errorf("ProxyPubsubService->SendMsg: sendMsgProtocol.Request error: %v", err)
		return nil, err
	}
	request, ok := requestProtoData.(*pb.MsgReq)
	if !ok {
		log.Errorf("ProxyPubsubService->SendMsg: requestProtoData is not MsgReq")
		return nil, fmt.Errorf("ProxyPubsubService->SendMsg: requestProtoData is not MsgReq")
	}
	log.Debugf("ProxyPubsubService->SendMsg end")
	return request, nil
}

func (d *ProxyPubsubService) SetOnReceiveMsg(onReceiveMsg msg.OnReceiveMsg) {
	d.onReceiveMsg = onReceiveMsg
}

func (d *ProxyPubsubService) SetOnSendMsgResponse(onSendMsgResponse msg.OnReceiveMsg) {
	d.onSendMsgResponse = onSendMsgResponse
}

// DmsgServiceInterface
func (d *ProxyPubsubService) GetPublishTarget(pubkey string) (*dmsgUser.Target, error) {
	log.Debugf("ProxyPubsubService->GetPublishTarget: need implement by inherit")
	return nil, nil
}

// MsgSpCallback
func (d *ProxyPubsubService) OnCreatePubsubRequest(
	requestProtoData protoreflect.ProtoMessage) (any, any, bool, error) {
	log.Debugf("ChannelService->OnCreatePubusubRequest begin:\nrequestProtoData: %+v", requestProtoData)

	request, ok := requestProtoData.(*pb.CreatePubsubReq)
	if !ok {
		log.Errorf("ChannelService->OnCreatePubusubRequest: fail to convert requestProtoData to *pb.CreateMailboxReq")
		return nil, nil, false, fmt.Errorf("ChannelService->OnCreatePubusubRequest: cannot convert to *pb.CreateMailboxReq")
	}
	channelKey := request.ChannelKey
	isAvailable := d.isAvailablePubsub(channelKey)
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

	err := d.SubscribePubsub(channelKey, false)
	if err != nil {
		return nil, nil, false, err
	}

	log.Debugf("ChannelService->OnCreatePubusubRequest end")
	return nil, nil, false, nil
}

func (d *ProxyPubsubService) OnCreatePubsubResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	log.Debugf("ProxyPubsubService->OnCreatePubsubResponse: need implement by inherit")
	return nil, nil
}

// MsgPpCallback
func (d *ProxyPubsubService) OnPubsubMsgRequest(
	requestProtoData protoreflect.ProtoMessage) (any, any, bool, error) {
	log.Debugf("ProxyPubsubService->OnPubsubMsgRequest: need implement by inherit")
	return nil, nil, false, nil
}

func (d *ProxyPubsubService) OnPubsubMsgResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	log.Debugf("ProxyPubsubService->OnPubsubMsgResponse: need implement by inherit")
	return nil, nil
}

// common
func (d *ProxyPubsubService) cleanRestResource() {
	go func() {
		ticker := time.NewTicker(3 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-d.stopCleanRestResource:
				return
			case <-ticker.C:
				for pubkey, pubsub := range d.proxyPubsubList {
					days := dmsgCommonUtil.DaysBetween(pubsub.LastReciveTimestamp, time.Now().UnixNano())
					if days >= d.GetKeepPubsubDay() {
						d.UnsubscribePubsub(pubkey)
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

func (d *ProxyPubsubService) GetKeepPubsubDay() int {
	log.Debugf("ProxyPubsubService->GetKeepPubsubDay: need implement by inherit")
	return 0
}

func (d *ProxyPubsubService) handlePubsubProtocol(target *dmsgUser.Target) error {
	ctx := d.TvBase.GetCtx()
	protocolDataChan, err := WaitMessage(ctx, target.Key.PubkeyHex)
	if err != nil {
		return err
	}
	log.Debugf("ProxyPubsubService->handlePubsubProtocol: protocolDataChan: %+v", protocolDataChan)

	go func() {
		for {
			select {
			case protocolHandle, ok := <-protocolDataChan:
				if !ok {
					return
				}
				pid := protocolHandle.PID
				log.Debugf("ProxyPubsubService->handlePubsubProtocol: \npid: %d\ntopicName: %s", pid, target.Pubsub.Topic.String())

				handle := d.ProtocolHandleList[pid]
				if handle == nil {
					log.Warnf("ProxyPubsubService->handlePubsubProtocol: no handle for pid: %d", pid)
					continue
				}
				msgRequestPID := d.pubsubMsgProtocol.Adapter.GetRequestPID()
				msgResponsePID := d.pubsubMsgProtocol.Adapter.GetResponsePID()
				data := protocolHandle.Data
				switch pid {
				case msgRequestPID:
					err = handle.HandleRequestData(data)
					if err != nil {
						log.Warnf("ProxyPubsubService->handlePubsubProtocol: HandleRequestData error: %v", err)
					}
					continue
				case msgResponsePID:
					err = handle.HandleResponseData(data)
					if err != nil {
						log.Warnf("ProxyPubsubService->handlePubsubProtocol: HandleResponseData error: %v", err)
					}
					continue
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return nil
}

func (d *ProxyPubsubService) CreatePubsubService(pubkey string) error {
	log.Debugf("ProxyPubsubService->CreatePubsubService begin:\n channel key: %s", pubkey)
	find := false

	hostId := d.TvBase.GetHost().ID().String()
	servicePeerList, _ := d.TvBase.GetAvailableServicePeerList(hostId)
	srcPubkey, err := d.GetUserPubkeyHex()
	if err != nil {
		log.Errorf("ProxyPubsubService->CreatePubsubService: GetUserPubkeyHex error: %v", err)
		return err
	}
	peerID := d.TvBase.GetHost().ID().String()
	for _, servicePeerID := range servicePeerList {
		log.Debugf("ProxyPubsubService->CreatePubsubService: servicePeerID: %s", servicePeerID)
		if peerID == servicePeerID.String() {
			continue
		}
		_, createPubsubDoneChan, err := d.createPubsubProtocol.Request(servicePeerID, srcPubkey, pubkey)
		if err != nil {
			continue
		}

		select {
		case responseProtoData := <-createPubsubDoneChan:
			log.Debugf("ProxyPubsubService->CreatePubsubService:\ncreateChannelResponseProtoData: %+v",
				responseProtoData)
			response, ok := responseProtoData.(*pb.CreatePubsubRes)
			if !ok || response == nil {
				log.Errorf("ProxyPubsubService->CreatePubsubService: createPubChannelResponseProtoData is not CreatePubChannelRes")
				continue
			}
			if response.RetCode.Code < 0 {
				log.Errorf("ProxyPubsubService->CreatePubsubService: createPubChannel fail")
				continue
			} else {
				log.Debugf("ProxyPubsubService->CreatePubsubService: createPubChannel success")
				find = true
				return nil
			}
		case <-time.After(time.Second * 3):
			continue
		case <-d.TvBase.GetCtx().Done():
			return fmt.Errorf("ProxyPubsubService->CreatePubsubService: BaseService.GetCtx().Done()")
		}
	}
	if !find {
		log.Error("ProxyPubsubService->CreatePubsubService: no available service peer")
		return fmt.Errorf("ProxyPubsubService->CreatePubsubService: no available service peer")
	}
	log.Debug("ProxyPubsubService->CreatePubsubService end")
	return nil
}

func (d *ProxyPubsubService) isAvailablePubsub(pubKey string) bool {
	log.Debugf("ProxyPubsubService->GetPublishTarget: need implement by inherit")
	return false
}