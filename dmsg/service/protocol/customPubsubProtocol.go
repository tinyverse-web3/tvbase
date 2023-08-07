package protocol

import (
	"context"
	"errors"
	"fmt"

	"github.com/libp2p/go-libp2p/core/host"
	tvLog "github.com/tinyverse-web3/tvbase/common/log"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
	customProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol/custom"
	"github.com/tinyverse-web3/tvbase/dmsg/service/common"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type CustomPubsubProtocolResponseParam struct {
	PID     string
	Service customProtocol.CustomPubsubProtocolService
}

type CustomPubsubProtocolAdapter struct {
	common.CommonProtocolAdapter
	protocol *common.PubsubProtocol
	pid      string
}

func NewCustomPubsubProtocolAdapter() *CustomPubsubProtocolAdapter {
	ret := &CustomPubsubProtocolAdapter{}
	return ret
}

func (adapter *CustomPubsubProtocolAdapter) init(customProtocolId string) {
	adapter.pid = customProtocolId
}

func (adapter *CustomPubsubProtocolAdapter) GetResponsePID() pb.PID {
	return pb.PID_CUSTOM_STREAM_RES
}

func (adapter *CustomPubsubProtocolAdapter) GetRequestPID() pb.PID {
	return pb.PID_CUSTOM_STREAM_REQ
}

func (adapter *CustomPubsubProtocolAdapter) GetEmptyRequest() protoreflect.ProtoMessage {
	return &pb.CustomProtocolReq{}
}
func (adapter *CustomPubsubProtocolAdapter) GetEmptyResponse() protoreflect.ProtoMessage {
	return &pb.CustomProtocolRes{}
}

func (adapter *CustomPubsubProtocolAdapter) GetRequestBasicData(requestProtoData protoreflect.ProtoMessage) *pb.BasicData {
	request, ok := requestProtoData.(*pb.CustomProtocolReq)
	if !ok {
		return nil
	}
	return request.BasicData
}

func (adapter *CustomPubsubProtocolAdapter) GetResponseBasicData(responseProtoData protoreflect.ProtoMessage) *pb.BasicData {
	response, ok := responseProtoData.(*pb.CustomProtocolRes)
	if !ok {
		return nil
	}
	return response.BasicData
}

func (adapter *CustomPubsubProtocolAdapter) InitResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseBasicData *pb.BasicData,
	dataList ...any) (protoreflect.ProtoMessage, error) {
	if len(dataList) < 1 {
		return nil, errors.New("CustomStreamProtocolAdapter:InitResponse: dataList need contain CustomPubsubProtocolResponseParam")
	}

	responseParam, ok := dataList[0].(*CustomPubsubProtocolResponseParam)
	if !ok {
		tvLog.Logger.Errorf("CustomPubsubProtocolAdapter->InitResponse: fail to cast dataList[0] to CustomPubsubProtocolResponseParam")
		return nil, fmt.Errorf("CustomPubsubProtocolAdapter->InitResponse: fail to cast dataList[0] to CustomPubsubProtocolResponseParam")
	}

	request, ok := requestProtoData.(*pb.CustomProtocolReq)
	if !ok {
		tvLog.Logger.Errorf("dmsgService->OnCustomPubsubProtocolResponse: cannot convert to *pb.CustomContentReq")
		return nil, fmt.Errorf("dmsgService->OnCustomPubsubProtocolResponse: cannot convert to *pb.CustomContentReq")
	}

	response := &pb.CustomProtocolRes{
		BasicData: responseBasicData,
		PID:       responseParam.PID,
		RetCode:   dmsgProtocol.NewSuccRetCode(),
	}

	// get response.Content
	err := responseParam.Service.HandleResponse(request, response)
	if err != nil {
		tvLog.Logger.Errorf("dmsgService->OnCustomPubsubProtocolResponse: Service.HandleResponse error: %v", err)
		return nil, err
	}

	return response, nil
}

func (adapter *CustomPubsubProtocolAdapter) SetResponseSig(responseProtoData protoreflect.ProtoMessage, sig []byte) error {
	response, ok := responseProtoData.(*pb.CustomProtocolRes)
	if !ok {
		return errors.New("CustomPubsubProtocolAdapter->SetResponseSig: failed to cast response to *pb.ReleaseMailboxRes")
	}
	response.BasicData.Sig = sig
	return nil
}

func (adapter *CustomPubsubProtocolAdapter) CallRequestCallback(requestProtoData protoreflect.ProtoMessage) (any, any, error) {
	data, retCode, err := adapter.protocol.Callback.OnCustomPubsubProtocolRequest(requestProtoData)
	return data, retCode, err
}

func (adapter *CustomPubsubProtocolAdapter) CallResponseCallback(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	data, err := adapter.protocol.Callback.OnCustomPubsubProtocolResponse(requestProtoData, responseProtoData)
	return data, err
}

func NewCustomPubsubProtocol(
	ctx context.Context,
	host host.Host,
	customProtocolId string,
	protocolService common.ProtocolService,
	protocolCallback common.PubsubProtocolCallback) *common.PubsubProtocol {
	adapter := NewCustomPubsubProtocolAdapter()
	protocol := common.NewPubsubProtocol(host, protocolService, protocolCallback, adapter)
	adapter.protocol = protocol
	adapter.init(customProtocolId)
	return protocol
}
