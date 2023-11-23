package customProtocol

import (
	"fmt"

	tvutilCrypto "github.com/tinyverse-web3/mtv_go_utils/crypto"
	tvutilKey "github.com/tinyverse-web3/mtv_go_utils/key"
	"github.com/tinyverse-web3/tvbase/common/define"
	dmsgCommonKey "github.com/tinyverse-web3/tvbase/dmsg/common/key"
	dmsgCommonService "github.com/tinyverse-web3/tvbase/dmsg/common/service"
	dmsgUser "github.com/tinyverse-web3/tvbase/dmsg/common/user"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/adapter"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/basic"
	dmsgProtocolCustom "github.com/tinyverse-web3/tvbase/dmsg/protocol/custom"
	dmsgServiceCommon "github.com/tinyverse-web3/tvbase/dmsg/service/common"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type CustomProtocolService struct {
	CustomProtocolBase
	queryPeerProtocol        *basic.QueryPeerProtocol
	serverStreamProtocolList map[string]*dmsgProtocolCustom.ServerStreamProtocol
	queryPeerTarget          *dmsgUser.Target
	stopServiceChan          chan bool
	enable                   bool
}

func NewService(tvbaseService define.TvBaseService, pubkey string, getSig dmsgCommonKey.GetSigCallback) (*CustomProtocolService, error) {
	d := &CustomProtocolService{}
	err := d.Init(tvbaseService, pubkey, getSig)
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (d *CustomProtocolService) Init(tvbaseService define.TvBaseService, pubkey string, getSig dmsgCommonKey.GetSigCallback) error {
	err := d.LightUserService.Init(tvbaseService)
	if err != nil {
		return err
	}
	err = d.LightUserService.Start(pubkey, getSig, false)
	if err != nil {
		return err
	}

	d.serverStreamProtocolList = make(map[string]*dmsgProtocolCustom.ServerStreamProtocol)
	d.stopServiceChan = make(chan bool)
	return nil
}

// sdk-common
func (d *CustomProtocolService) Start() error {
	log.Debug("CustomProtocolService->Start begin")
	ctx := d.TvBase.GetCtx()
	host := d.TvBase.GetHost()

	if d.queryPeerProtocol == nil {
		d.queryPeerProtocol = adapter.NewQueryPeerProtocol(ctx, host, d, d)
		d.RegistPubsubProtocol(d.queryPeerProtocol.Adapter.GetResponsePID(), d.queryPeerProtocol)
		d.RegistPubsubProtocol(d.queryPeerProtocol.Adapter.GetRequestPID(), d.queryPeerProtocol)
	}
	if d.queryPeerTarget == nil {
		topicName := dmsgCommonService.GetQueryPeerTopicName()
		topicNamePrikey, topicNamePubkey, err := tvutilKey.GenerateEcdsaKey(topicName)
		if err != nil {
			return err
		}
		topicNamePubkeyData, err := tvutilKey.ECDSAPublicKeyToProtoBuf(topicNamePubkey)
		if err != nil {
			log.Errorf("initDmsg: ECDSAPublicKeyToProtoBuf error: %v", err)
			return err
		}
		topicNamePubkeyHex := tvutilKey.TranslateKeyProtoBufToString(topicNamePubkeyData)

		topicNameGetSig := func(protoData []byte) ([]byte, error) {
			sig, err := tvutilCrypto.SignDataByEcdsa(topicNamePrikey, protoData)
			if err != nil {
				log.Errorf("initDmsg: sig error: %v", err)
			}
			return sig, nil
		}

		d.queryPeerTarget, err = dmsgUser.NewTarget(topicNamePubkeyHex, topicNameGetSig)
		if err != nil {
			return err
		}
		err = d.queryPeerTarget.InitPubsub(topicNamePubkeyHex)
		if err != nil {
			return err
		}
		err = d.HandlePubsubProtocol(d.queryPeerTarget)
		if err != nil {
			log.Errorf("CustomProtocolService->Start: HandlePubsubProtocol error: %v", err)
			return err
		}
	}

	d.enable = true
	log.Debug("CustomProtocolService->Start end")
	return nil
}

func (d *CustomProtocolService) Stop() error {
	select {
	case d.stopServiceChan <- true:
		log.Debugf("CustomProtocolService->Stop: succ send stopService")
	default:
		log.Debugf("CustomProtocolService->Stop: no receiver for stopService")
	}
	d.enable = false
	return nil
}

func (d *CustomProtocolService) Release() error {
	d.UnregistPubsubProtocol(d.queryPeerProtocol.Adapter.GetResponsePID())
	d.UnregistPubsubProtocol(d.queryPeerProtocol.Adapter.GetRequestPID())
	d.queryPeerProtocol = nil

	err := d.Stop()
	if err != nil {
		return err
	}
	err = d.queryPeerTarget.Close()
	if err != nil {
		return err
	}
	d.queryPeerTarget = nil
	err = d.LightUserService.Stop()
	if err != nil {
		return err
	}
	close(d.stopServiceChan)
	return nil
}

func (d *CustomProtocolService) RegistServer(service dmsgProtocolCustom.ServerHandle) error {
	customProtocolID := service.GetProtocolID()
	if d.serverStreamProtocolList[customProtocolID] != nil {
		log.Errorf("CustomProtocolService->RegistCSPServer: protocol %s is already exist", customProtocolID)
		return fmt.Errorf("CustomProtocolService->RegistCSPServer: protocol %s is already exist", customProtocolID)
	}
	d.serverStreamProtocolList[customProtocolID] = &dmsgProtocolCustom.ServerStreamProtocol{
		Protocol: adapter.NewCustomStreamProtocol(d.TvBase.GetCtx(), d.TvBase.GetHost(), customProtocolID, d, d, true),
		Handle:   service,
	}
	service.SetCtx(d.TvBase.GetCtx())
	return nil
}

func (d *CustomProtocolService) UnregistServer(callback dmsgProtocolCustom.ServerHandle) error {
	customProtocolID := callback.GetProtocolID()
	if d.serverStreamProtocolList[customProtocolID] == nil {
		log.Warnf("CustomProtocolService->UnregistCSPServer: protocol %s is not exist", customProtocolID)
		return nil
	}
	d.serverStreamProtocolList[customProtocolID] = nil
	return nil
}

func (d *CustomProtocolService) OnCustomRequest(
	requestProtoData protoreflect.ProtoMessage) (any, any, bool, error) {
	log.Debugf("CustomProtocolService->OnCustomRequest begin")
	if !d.enable {
		return nil, nil, true, nil
	}
	request, ok := requestProtoData.(*pb.CustomProtocolReq)
	if !ok {
		log.Errorf("CustomProtocolService->OnCustomRequest: fail to convert requestProtoData to *pb.CustomContentReq")
		return nil, nil, false, fmt.Errorf("CustomProtocolService->OnCustomRequest: fail to convert requestProtoData to *pb.CustomContentReq")
	}

	log.Debugf("dmsgService->OnCustomRequest:\nrequest.BasicData: %v\nrequest.PID: ", request.BasicData, request.PID)

	customProtocolInfo := d.serverStreamProtocolList[request.PID]
	if customProtocolInfo == nil {
		log.Errorf("CustomProtocolService->OnCustomRequest: customProtocolInfo is nil, request: %+v", request)
		return nil, nil, false, fmt.Errorf("CustomProtocolService->OnCustomRequest: customProtocolInfo is nil, request: %+v", request)
	}
	responseContent, retCode, err := customProtocolInfo.Handle.HandleRequest(request)
	if err != nil {
		return nil, nil, true, err
	}

	log.Debugf("CustomProtocolService->OnRequest end")
	return responseContent, retCode, false, nil
}

func (d *CustomProtocolService) OnQueryPeerRequest(requestProtoData protoreflect.ProtoMessage) (any, any, bool, error) {
	log.Debugf("CustomProtocolService->OnQueryPeerRequest begin\nrequestProtoData: %+v", requestProtoData)
	if !d.enable {
		return nil, nil, true, nil
	}
	request, ok := requestProtoData.(*pb.QueryPeerReq)
	if !ok {
		log.Errorf("CustomProtocolService->OnQueryPeerRequest: fail to convert requestProtoData to *pb.QueryPeerReq")
		return nil, nil, false, fmt.Errorf("CustomProtocolService->OnQueryPeerRequest: fail to convert requestProtoData to *pb.QueryPeerReq")
	}

	if d.serverStreamProtocolList[request.Pid] == nil {
		log.Errorf("CustomProtocolService->OnQueryPeerRequest: pid %s is not exist", request.Pid)
		return nil, nil, true, fmt.Errorf("CustomProtocolService->OnQueryPeerRequest: pid %s is not exist", request.Pid)
	}

	log.Debug("CustomProtocolService->OnQueryPeerRequest end")
	return d.TvBase.GetHost().ID().String(), nil, false, nil
}

func (d *CustomProtocolService) HandlePubsubProtocol(target *dmsgUser.Target) error {
	ctx := d.TvBase.GetCtx()
	protocolDataChan, err := dmsgServiceCommon.WaitMessage(ctx, target.Key.PubkeyHex)
	if err != nil {
		return err
	}
	log.Debugf("CustomProtocolService->HandlePubsubProtocol: protocolDataChan: %+v", protocolDataChan)

	go func() {
		for {
			select {
			case <-d.stopServiceChan:
				return
			case protocolHandle, ok := <-protocolDataChan:
				if !ok {
					return
				}
				pid := protocolHandle.PID
				log.Debugf("CustomProtocolService->HandlePubsubProtocol: \npid: %d\ntopicName: %s", pid, target.Pubsub.Topic.String())

				handle := d.ProtocolHandleList[pid]
				if handle == nil {
					log.Warnf("CustomProtocolService->HandlePubsubProtocol: no handle for pid: %d", pid)
					continue
				}
				msgRequestPID := d.queryPeerProtocol.Adapter.GetRequestPID()
				msgResponsePID := d.queryPeerProtocol.Adapter.GetResponsePID()
				data := protocolHandle.Data
				switch pid {
				case msgRequestPID:
					log.Debugf("CustomProtocolService->HandlePubsubProtocol: protocolDataChan: %+v", protocolDataChan)
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
