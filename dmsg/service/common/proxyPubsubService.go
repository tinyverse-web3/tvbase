package common

import (
	"errors"
	"fmt"
	"time"

	ipfsLog "github.com/ipfs/go-log/v2"
	"github.com/tinyverse-web3/tvbase/common/define"
	dmsgKey "github.com/tinyverse-web3/tvbase/dmsg/common/key"

	"github.com/tinyverse-web3/tvbase/dmsg/common/msg"
	dmsgUser "github.com/tinyverse-web3/tvbase/dmsg/common/user"
	dmsgCommonUtil "github.com/tinyverse-web3/tvbase/dmsg/common/util"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var log = ipfsLog.Logger("dmsg.service.proxypubsub")

type ProxyPubsubService struct {
	LightUserService
	createPubsubProtocol  *dmsgProtocol.CreatePubsubSProtocol
	pubsubMsgProtocol     *dmsgProtocol.PubsubMsgProtocol
	PubsubList            map[string]*dmsgUser.ProxyPubsub
	OnReceiveMsg          msg.OnReceiveMsg
	OnResponseMsg         msg.OnRespondMsg
	stopCleanRestResource chan bool
	maxPubsubCount        int
	keepPubsubDay         int
}

func (d *ProxyPubsubService) Init(
	tvbase define.TvBaseService,
	maxPubsubCount int,
	keepPubsubDay int) error {
	err := d.LightUserService.Init(tvbase)
	if err != nil {
		return err
	}
	d.maxPubsubCount = maxPubsubCount
	d.keepPubsubDay = keepPubsubDay
	d.PubsubList = make(map[string]*dmsgUser.ProxyPubsub)
	return nil
}

// sdk-common
func (d *ProxyPubsubService) Start(
	pubkeyData []byte,
	getSig dmsgKey.GetSigCallback,
	createPubsubProtocol *dmsgProtocol.CreatePubsubSProtocol,
	pubsubMsgProtocol *dmsgProtocol.PubsubMsgProtocol,
	isListenPubsubMsg bool,
) error {
	log.Debug("ProxyPubsubService->Start begin")
	// stream protocol
	d.createPubsubProtocol = createPubsubProtocol

	// pubsub protocol
	d.pubsubMsgProtocol = pubsubMsgProtocol

	// light user
	err := d.LightUserService.Start(pubkeyData, getSig, isListenPubsubMsg)
	if err != nil {
		return err
	}

	if isListenPubsubMsg {
		err = d.HandlePubsubProtocol(&d.LightUser.Target)
		if err != nil {
			log.Errorf("ProxyPubsubService->Start: HandlePubsubProtocol error: %v", err)
			return err
		}
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
	return d.PubsubList[pubkey]
}

func (d *ProxyPubsubService) SubscribePubsub(
	pubkey string,
	createPubsubProxy bool,
	isHandlePubsubProtocol bool,
	autoClean bool,
) error {
	log.Debugf("ProxyPubsubService->SubscribePubsub begin:\npubkey: %s", pubkey)

	if d.PubsubList[pubkey] != nil {
		log.Errorf("ProxyPubsubService->SubscribePubsub: pubkey is already exist in ProxyPubsubList")
		return fmt.Errorf("ProxyPubsubService->SubscribePubsub: pubkey is already exist in ProxyPubsubList")
	}

	target, err := dmsgUser.NewTarget(pubkey, nil)
	if err != nil {
		log.Errorf("ProxyPubsubService->SubscribePubsub: NewTarget error: %v", err)
		return err
	}

	err = target.InitPubsub(pubkey)
	if err != nil {
		log.Errorf("ProxyPubsubService->SubscribePubsub: target.InitPubsub error: %v", err)
		return err
	}

	proxyPubsub := &dmsgUser.ProxyPubsub{
		DestTarget: dmsgUser.DestTarget{
			Target:        *target,
			LastTimestamp: time.Now().UnixNano(),
		},
		AutoClean: autoClean,
	}

	if createPubsubProxy {
		err = d.createPubsubService(proxyPubsub.Key.PubkeyHex)
		if err != nil {
			target.Close()
			return err
		}
	}

	if isHandlePubsubProtocol {
		d.HandlePubsubProtocol(&proxyPubsub.Target)
		if err != nil {
			log.Errorf("ProxyPubsubService->SubscribePubsub: HandlePubsubProtocol error: %v", err)
			err := proxyPubsub.Target.Close()
			if err != nil {
				log.Warnf("ProxyPubsubService->SubscribePubsub: Target.Close error: %v", err)
				return err
			}
			return err
		}
	}

	d.PubsubList[pubkey] = proxyPubsub
	log.Debug("ProxyPubsubService->SubscribePubsub end")
	return nil
}

func (d *ProxyPubsubService) UnsubscribePubsub(pubkey string) error {
	log.Debugf("ProxyPubsubService->UnsubscribePubsub begin\npubKey: %s", pubkey)

	proxyPubsub := d.PubsubList[pubkey]
	if proxyPubsub == nil {
		log.Errorf("ProxyPubsubService->UnsubscribePubsub: proxyPubsub is nil")
		return fmt.Errorf("ProxyPubsubService->UnsubscribePubsub: proxyPubsub is nil")
	}
	err := proxyPubsub.Close()
	if err != nil {
		log.Warnf("ProxyPubsubService->UnsubscribePubsub: proxyPubsub.Close error: %v", err)
	}
	delete(d.PubsubList, pubkey)

	log.Debug("ProxyPubsubService->UnsubscribePubsub end")
	return nil
}

func (d *ProxyPubsubService) UnsubscribePubsubList() error {
	for pubkey := range d.PubsubList {
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

func (d *ProxyPubsubService) SetOnReceiveMsg(cb msg.OnReceiveMsg) {
	d.OnReceiveMsg = cb
}

func (d *ProxyPubsubService) SetOnMsgResponse(cb msg.OnRespondMsg) {
	d.OnResponseMsg = cb
}

// MsgSpCallback
func (d *ProxyPubsubService) OnCreatePubsubRequest(
	requestProtoData protoreflect.ProtoMessage) (any, any, bool, error) {
	log.Debugf("ProxyPubsubService->OnCreatePubusubRequest begin:\nrequestProtoData: %+v", requestProtoData)

	request, ok := requestProtoData.(*pb.CreatePubsubReq)
	if !ok {
		log.Errorf("ProxyPubsubService->OnCreatePubusubRequest: fail to convert requestProtoData to *pb.CreatePubsubReq")
		return nil, nil, false, fmt.Errorf("ProxyPubsubService->OnCreatePubusubRequest: cannot convert to *pb.CreatePubsubReq")
	}

	if request.BasicData.PeerID == d.TvBase.GetHost().ID().String() {
		log.Debugf("ProxyPubsubService->OnCreatePubusubRequest: request.BasicData.PeerID == d.TvBase.GetDht().PeerID().String()")
		return nil, nil, true, nil
	}

	pubsubKey := request.Key
	isAvailable := d.isAvailablePubsub()
	if !isAvailable {
		return nil, nil, false, errors.New("ProxyPubsubService->OnCreatePubusubRequest: exceeded the maximum number of mailbox service")
	}
	pubsub := d.PubsubList[pubsubKey]
	if pubsub != nil {
		log.Debugf("ProxyPubsubService->OnCreatePubusubRequest: pubsub already exist")
		retCode := &pb.RetCode{
			Code:   dmsgProtocol.AlreadyExistCode,
			Result: "ProxyPubsubService->OnCreatePubusubRequest: pubsub already exist",
		}
		return nil, retCode, false, nil
	}

	err := d.SubscribePubsub(pubsubKey, false, false, true)
	if err != nil {
		return nil, nil, false, err
	}

	log.Debug("ProxyPubsubService->OnCreatePubusubRequest end")
	return nil, nil, false, nil
}

func (d *ProxyPubsubService) OnCreatePubsubResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	return nil, nil
}

func (d *ProxyPubsubService) HandlePubsubProtocol(target *dmsgUser.Target) error {
	ctx := d.TvBase.GetCtx()
	protocolDataChan, err := WaitMessage(ctx, target.Key.PubkeyHex)
	if err != nil {
		return err
	}
	log.Debugf("ProxyPubsubService->HandlePubsubProtocol: protocolDataChan: %+v", protocolDataChan)

	go func() {
		for {
			select {
			case protocolHandle, ok := <-protocolDataChan:
				if !ok {
					return
				}
				pid := protocolHandle.PID
				log.Debugf("ProxyPubsubService->HandlePubsubProtocol: \npid: %d\ntopicName: %s", pid, target.Pubsub.Topic.String())

				handle := d.ProtocolHandleList[pid]
				if handle == nil {
					log.Warnf("ProxyPubsubService->HandlePubsubProtocol: no handle for pid: %d", pid)
					continue
				}
				msgRequestPID := d.pubsubMsgProtocol.Adapter.GetRequestPID()
				msgResponsePID := d.pubsubMsgProtocol.Adapter.GetResponsePID()
				data := protocolHandle.Data
				switch pid {
				case msgRequestPID:
					log.Debugf("ProxyPubsubService->HandlePubsubProtocol: protocolDataChan: %+v", protocolDataChan)
					handle.HandleRequestData(data)
					continue
				case msgResponsePID:
					handle.HandleResponseData(data)
					continue
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return nil
}

func (d *ProxyPubsubService) createPubsubService(pubkey string) error {
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
		_, createPubsubResponseChan, err := d.createPubsubProtocol.Request(servicePeerID, srcPubkey, pubkey)
		if err != nil {
			continue
		}

		select {
		case responseProtoData := <-createPubsubResponseChan:
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

func (d *ProxyPubsubService) CleanRestPubsub(dur time.Duration) {
	go func() {
		d.stopCleanRestResource = make(chan bool)
		ticker := time.NewTicker(dur)
		defer ticker.Stop()
		for {
			select {
			case <-d.stopCleanRestResource:
				return
			case <-ticker.C:
				for pubkey, pubsub := range d.PubsubList {
					days := dmsgCommonUtil.DaysBetween(pubsub.LastTimestamp, time.Now().UnixNano())
					if pubsub.AutoClean && days >= d.keepPubsubDay {
						d.UnsubscribePubsub(pubkey)
						continue
					}
				}
				continue
			case <-d.TvBase.GetCtx().Done():
				return
			}
		}
	}()
}

func (d *ProxyPubsubService) isAvailablePubsub() bool {
	len := len(d.PubsubList)
	if len >= d.maxPubsubCount {
		log.Warnf("ProxyPubsubService->isAvailablePubsub: exceeded the maximum number of mailbox services, current destUserCount:%v", len)
		return false
	}
	return true
}
