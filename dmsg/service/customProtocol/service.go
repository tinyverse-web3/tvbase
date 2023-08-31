package customProtocol

import (
	"fmt"
	"time"

	ipfsLog "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/peer"
	tvbaseCommon "github.com/tinyverse-web3/tvbase/common"
	dmsgCommonKey "github.com/tinyverse-web3/tvbase/dmsg/common/key"
	dmsgCommonService "github.com/tinyverse-web3/tvbase/dmsg/common/service"
	dmsgUser "github.com/tinyverse-web3/tvbase/dmsg/common/user"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/adapter"
	dmsgProtocolCustom "github.com/tinyverse-web3/tvbase/dmsg/protocol/custom"
	dmsgServiceCommon "github.com/tinyverse-web3/tvbase/dmsg/service/common"
	tvutilCrypto "github.com/tinyverse-web3/tvutil/crypto"
	tvutilKey "github.com/tinyverse-web3/tvutil/key"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var log = ipfsLog.Logger("dmsg.service.customprotocol")

type CustomProtocolService struct {
	dmsgServiceCommon.LightUserService
	queryPeerProtocol        *dmsgProtocol.QueryPeerProtocol
	serverStreamProtocolList map[string]*dmsgProtocolCustom.ServerStreamProtocol
	clientStreamProtocolList map[string]*dmsgProtocolCustom.ClientStreamProtocol
	queryPeerTarget          *dmsgUser.Target
}

func CreateService(tvbaseService tvbaseCommon.TvBaseService) (*CustomProtocolService, error) {
	d := &CustomProtocolService{}
	err := d.Init(tvbaseService)
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (d *CustomProtocolService) Init(tvbaseService tvbaseCommon.TvBaseService) error {
	err := d.LightUserService.Init(tvbaseService)
	if err != nil {
		return err
	}
	d.clientStreamProtocolList = make(map[string]*dmsgProtocolCustom.ClientStreamProtocol)
	d.serverStreamProtocolList = make(map[string]*dmsgProtocolCustom.ServerStreamProtocol)

	return nil
}

// sdk-common
func (d *CustomProtocolService) Start(
	enableService bool,
	pubkeyData []byte,
	getSig dmsgCommonKey.GetSigCallback,
	timeout time.Duration,
) error {
	log.Debug("CustomProtocolService->Start begin")
	err := d.LightUserService.Start(enableService, pubkeyData, getSig, false)
	if err != nil {
		return err
	}

	ctx := d.TvBase.GetCtx()
	host := d.TvBase.GetHost()

	d.queryPeerProtocol = adapter.NewQueryPeerProtocol(ctx, host, d, d)
	d.RegistPubsubProtocol(d.queryPeerProtocol.Adapter.GetResponsePID(), d.queryPeerProtocol)

	if enableService {
		d.RegistPubsubProtocol(d.queryPeerProtocol.Adapter.GetRequestPID(), d.queryPeerProtocol)
	}

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

	log.Debug("CustomProtocolService->Start end")
	return nil
}

func (d *CustomProtocolService) Stop() error {
	log.Debug("CustomProtocolService->Stop begin")
	err := d.LightUserService.Stop()
	if err != nil {
		return err
	}
	log.Debug("CustomProtocolService->Stop end")
	return nil
}

func (d *CustomProtocolService) GetQueryPeerPubkey() string {
	topicName := dmsgCommonService.GetQueryPeerTopicName()
	_, topicNamePubkey, err := tvutilKey.GenerateEcdsaKey(topicName)
	if err != nil {
		return ""
	}
	topicNamePubkeyData, err := tvutilKey.ECDSAPublicKeyToProtoBuf(topicNamePubkey)
	if err != nil {
		log.Errorf("CustomProtocolService->GetQueryPeerPubkey: ECDSAPublicKeyToProtoBuf error: %v", err)
		return ""
	}
	topicNamePubkeyHex := tvutilKey.TranslateKeyProtoBufToString(topicNamePubkeyData)
	return topicNamePubkeyHex
}

func (d *CustomProtocolService) QueryPeer(pid string) (*pb.QueryPeerReq, chan any, error) {
	destPubkey := d.GetQueryPeerPubkey()
	request, responseChan, err := d.queryPeerProtocol.Request(d.LightUser.Key.PubkeyHex, destPubkey, pid)
	return request.(*pb.QueryPeerReq), responseChan, err
}

func (d *CustomProtocolService) Request(
	peerIdStr string,
	pid string,
	content []byte,
) (*pb.CustomProtocolReq, chan any, error) {
	protocolInfo := d.clientStreamProtocolList[pid]
	if protocolInfo == nil {
		log.Errorf("CustomProtocolService->RequestService: protocol %s is not exist", pid)
		return nil, nil, fmt.Errorf("CustomProtocolService->RequestService: protocol %s is not exist", pid)
	}

	peerID, err := peer.Decode(peerIdStr)
	if err != nil {
		log.Errorf("CustomProtocolService->RequestService: err: %v", err)
		return nil, nil, err
	}
	request, responseChan, err := protocolInfo.Protocol.Request(
		peerID,
		d.LightUser.Key.PubkeyHex,
		pid,
		content)
	if err != nil {
		log.Errorf("CustomProtocolService->RequestService: err: %v, servicePeerInfo: %v, user public key: %s, content: %v",
			err, peerID, d.LightUser.Key.PubkeyHex, content)
		return nil, nil, err
	}
	return request.(*pb.CustomProtocolReq), responseChan, nil
}

func (d *CustomProtocolService) RegistClient(client dmsgProtocolCustom.ClientHandle) error {
	customProtocolID := client.GetProtocolID()
	if d.clientStreamProtocolList[customProtocolID] != nil {
		log.Errorf("CustomProtocolService->RegistCSPClient: protocol %s is already exist", customProtocolID)
		return fmt.Errorf("CustomProtocolService->RegistCSPClient: protocol %s is already exist", customProtocolID)
	}
	d.clientStreamProtocolList[customProtocolID] = &dmsgProtocolCustom.ClientStreamProtocol{
		Protocol: adapter.NewCustomStreamProtocol(d.TvBase.GetCtx(), d.TvBase.GetHost(), customProtocolID, d, d, d.EnableService),
		Handle:   client,
	}
	client.SetCtx(d.TvBase.GetCtx())
	client.SetService(d)
	return nil
}

func (d *CustomProtocolService) UnregistClient(client dmsgProtocolCustom.ClientHandle) error {
	customProtocolID := client.GetProtocolID()
	if d.clientStreamProtocolList[customProtocolID] == nil {
		log.Warnf("CustomProtocolService->UnregistCSPClient: protocol %s is not exist", customProtocolID)
		return nil
	}
	d.clientStreamProtocolList[customProtocolID] = nil
	return nil
}

func (d *CustomProtocolService) RegistServer(service dmsgProtocolCustom.ServerHandle) error {
	customProtocolID := service.GetProtocolID()
	if d.serverStreamProtocolList[customProtocolID] != nil {
		log.Errorf("CustomProtocolService->RegistCSPServer: protocol %s is already exist", customProtocolID)
		return fmt.Errorf("CustomProtocolService->RegistCSPServer: protocol %s is already exist", customProtocolID)
	}
	d.serverStreamProtocolList[customProtocolID] = &dmsgProtocolCustom.ServerStreamProtocol{
		Protocol: adapter.NewCustomStreamProtocol(d.TvBase.GetCtx(), d.TvBase.GetHost(), customProtocolID, d, d, d.EnableService),
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
	log.Debugf("CustomProtocolService->OnCustomRequest begin:\nrequestProtoData: %+v", requestProtoData)
	request, ok := requestProtoData.(*pb.CustomProtocolReq)
	if !ok {
		log.Errorf("CustomProtocolService->OnCustomRequest: fail to convert requestProtoData to *pb.CustomContentReq")
		return nil, nil, false, fmt.Errorf("CustomProtocolService->OnCustomRequest: fail to convert requestProtoData to *pb.CustomContentReq")
	}

	customProtocolInfo := d.serverStreamProtocolList[request.PID]
	if customProtocolInfo == nil {
		log.Errorf("CustomProtocolService->OnCustomRequest: customProtocolInfo is nil, request: %+v", request)
		return nil, nil, false, fmt.Errorf("CustomProtocolService->OnCustomRequest: customProtocolInfo is nil, request: %+v", request)
	}
	err := customProtocolInfo.Handle.HandleRequest(request)
	if err != nil {
		return nil, nil, false, err
	}
	param := &dmsgProtocolCustom.ResponseParam{
		PID:     request.PID,
		Service: customProtocolInfo.Handle,
	}

	log.Debugf("CustomProtocolService->OnRequest end")
	return param, nil, false, nil
}

func (d *CustomProtocolService) OnCustomResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	log.Debugf(
		"CustomProtocolService->OnCustomResponse begin:\nrequestProtoData: %+v\nresponseProtoData: %+v",
		requestProtoData, responseProtoData)

	return nil, nil
}

func (d *CustomProtocolService) OnQueryPeerRequest(requestProtoData protoreflect.ProtoMessage) (any, any, bool, error) {
	log.Debugf("CustomProtocolService->OnQueryPeerRequest begin\nrequestProtoData: %+v", requestProtoData)
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

func (d *CustomProtocolService) GetPublishTarget(pubkey string) (*dmsgUser.Target, error) {
	if d.queryPeerTarget == nil {
		log.Errorf("CustomProtocolService->GetPublishTarget: queryPeerTarget is nil")
		return nil, fmt.Errorf("CustomProtocolService->GetPublishTarget: queryPeerTarget is nil")
	}
	return d.queryPeerTarget, nil
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
