package customProtocol

import (
	"fmt"

	ipfsLog "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/peer"
	tvbaseCommon "github.com/tinyverse-web3/tvbase/common"
	dmsgCommonKey "github.com/tinyverse-web3/tvbase/dmsg/common/key"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/adapter"
	dmsgProtocolCustom "github.com/tinyverse-web3/tvbase/dmsg/protocol/custom"
	dmsgServiceCommon "github.com/tinyverse-web3/tvbase/dmsg/service/common"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var log = ipfsLog.Logger("dmsg.service.customprotocol")

type CustomProtocolService struct {
	dmsgServiceCommon.LightUserService

	serverStreamProtocolList map[string]*dmsgProtocolCustom.ServerStreamProtocol
	clientStreamProtocolList map[string]*dmsgProtocolCustom.ClientStreamProtocol
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
	EnableService bool,
	pubkeyData []byte,
	getSig dmsgCommonKey.GetSigCallback) error {
	log.Debug("CustomProtocolService->Start begin")
	err := d.LightUserService.Start(EnableService, pubkeyData, getSig, false)
	if err != nil {
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

// sdk-custom-stream-protocol
func (d *CustomProtocolService) Request(peerIdStr string, pid string, content []byte) error {
	protocolInfo := d.clientStreamProtocolList[pid]
	if protocolInfo == nil {
		log.Errorf("CustomProtocolService->Request: protocol %s is not exist", pid)
		return fmt.Errorf("CustomProtocolService->Request: protocol %s is not exist", pid)
	}

	peerID, err := peer.Decode(peerIdStr)
	if err != nil {
		log.Errorf("CustomProtocolService->Request: err: %v", err)
		return err
	}
	_, _, err = protocolInfo.Protocol.Request(
		peerID,
		d.LightUser.Key.PubkeyHex,
		pid,
		content)
	if err != nil {
		log.Errorf("CustomProtocolService->Request: err: %v, servicePeerInfo: %v, user public key: %s, content: %v",
			err, peerID, d.LightUser.Key.PubkeyHex, content)
		return err
	}
	return nil
}

func (d *CustomProtocolService) RegistClient(client dmsgProtocolCustom.ClientHandle) error {
	customProtocolID := client.GetProtocolID()
	if d.clientStreamProtocolList[customProtocolID] != nil {
		log.Errorf("CustomProtocolService->RegistCSPClient: protocol %s is already exist", customProtocolID)
		return fmt.Errorf("CustomProtocolService->RegistCSPClient: protocol %s is already exist", customProtocolID)
	}
	d.clientStreamProtocolList[customProtocolID] = &dmsgProtocolCustom.ClientStreamProtocol{
		Protocol: adapter.NewCustomStreamProtocol(d.TvBase.GetCtx(), d.TvBase.GetHost(), customProtocolID, d, d),
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
		Protocol: adapter.NewCustomStreamProtocol(d.TvBase.GetCtx(), d.TvBase.GetHost(), customProtocolID, d, d),
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

// MsgSpCallback
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

	request, ok := requestProtoData.(*pb.CustomProtocolReq)
	if !ok {
		log.Errorf("CustomProtocolService->OnCustomResponse: fail to convert requestProtoData to *pb.CustomContentReq")
		return nil, fmt.Errorf("CustomProtocolService->OnCustomResponse: fail to convert requestProtoData to *pb.CustomContentReq")
	}
	response, ok := responseProtoData.(*pb.CustomProtocolRes)
	if !ok {
		log.Errorf("CustomProtocolService->OnCustomResponse: fail to convert requestProtoData to *pb.CustomContentRes")
		return nil, fmt.Errorf("CustomProtocolService->OnCustomResponse: fail to convert requestProtoData to *pb.CustomContentRes")
	}

	customProtocolInfo := d.clientStreamProtocolList[response.PID]
	if customProtocolInfo == nil {
		log.Errorf("CustomProtocolService->OnCustomResponse: customProtocolInfo is nil, response: %+v", response)
		return nil, fmt.Errorf("CustomProtocolService->OnCustomResponse: customProtocolInfo is nil, response: %+v", response)
	}
	if customProtocolInfo.Handle == nil {
		log.Errorf("CustomProtocolService->OnCustomResponse: customProtocolInfo.Client is nil")
		return nil, fmt.Errorf("CustomProtocolService->OnCustomResponse: customProtocolInfo.Client is nil")
	}

	err := customProtocolInfo.Handle.HandleResponse(request, response)
	if err != nil {
		log.Errorf("CustomProtocolService->OnCustomResponse: Client.HandleResponse error: %v", err)
		return nil, err
	}
	return nil, nil
}
