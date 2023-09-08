package protocol

import (
	"context"
	"errors"
	"fmt"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"
	tvLog "github.com/tinyverse-web3/tvbase/common/log"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/service/common"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type CustomStreamProtocolAdapter struct {
	common.CommonProtocolAdapter
	protocol *common.StreamProtocol
	pid      string
}

func NewCustomStreamProtocolAdapter() *CustomStreamProtocolAdapter {
	ret := &CustomStreamProtocolAdapter{}
	return ret
}

func (adapter *CustomStreamProtocolAdapter) init() {
}

func (adapter *CustomStreamProtocolAdapter) GetResponsePID() pb.PID {
	return pb.PID_CUSTOM_STREAM_RES
}

func (adapter *CustomStreamProtocolAdapter) GetRequestPID() pb.PID {
	return pb.PID_CUSTOM_STREAM_REQ
}

func (adapter *CustomStreamProtocolAdapter) GetStreamResponsePID() protocol.ID {
	return protocol.ID(dmsgProtocol.PidCustomProtocolRes + "/" + adapter.pid)
}

func (adapter *CustomStreamProtocolAdapter) GetStreamRequestPID() protocol.ID {
	return protocol.ID(dmsgProtocol.PidCustomProtocolReq + "/" + adapter.pid)
}

func (adapter *CustomStreamProtocolAdapter) GetEmptyRequest() protoreflect.ProtoMessage {
	return &pb.CustomProtocolReq{}
}
func (adapter *CustomStreamProtocolAdapter) GetEmptyResponse() protoreflect.ProtoMessage {
	return &pb.CustomProtocolRes{}
}

func (adapter *CustomStreamProtocolAdapter) GetRequestBasicData(requestProtoData protoreflect.ProtoMessage) *pb.BasicData {
	request, ok := requestProtoData.(*pb.CustomProtocolReq)
	if !ok {
		return nil
	}
	return request.BasicData
}

func (adapter *CustomStreamProtocolAdapter) GetResponseBasicData(responseProtoData protoreflect.ProtoMessage) *pb.BasicData {
	response, ok := responseProtoData.(*pb.CustomProtocolRes)
	if !ok {
		return nil
	}
	return response.BasicData
}

func (adapter *CustomStreamProtocolAdapter) InitResponse(
	requestProtoData protoreflect.ProtoMessage,
	basicData *pb.BasicData,
	dataList ...any) (protoreflect.ProtoMessage, error) {
	if len(dataList) < 1 {
		return nil, errors.New("CustomStreamProtocolAdapter:InitResponse: dataList need contain customStreamProtocolResponseParam")
	}

	request, ok := requestProtoData.(*pb.CustomProtocolReq)
	if !ok {
		tvLog.Logger.Errorf("CustomStreamProtocolAdapter->InitResponse: fail to cast requestProtoData to *pb.CustomContentReq")
		return nil, fmt.Errorf("CustomStreamProtocolAdapter->InitResponse: fail to cast requestProtoData to *pb.CustomContentReq")
	}

	content, ok := dataList[0].([]byte)
	if !ok {
		tvLog.Logger.Errorf("CustomStreamProtocolAdapter->InitResponse: fail to cast dataList[0] to []byte")
		return nil, fmt.Errorf("CustomStreamProtocolAdapter->InitResponse: fail to cast dataList[0] to []byte")
	}

	retCode, ok := dataList[1].(*pb.RetCode)
	if !ok {
		retCode = dmsgProtocol.NewSuccRetCode()
	}

	response := &pb.CustomProtocolRes{
		BasicData: basicData,
		PID:       request.PID,
		Content:   content,
		RetCode:   retCode,
	}

	return response, nil
}

func (adapter *CustomStreamProtocolAdapter) SetResponseSig(responseProtoData protoreflect.ProtoMessage, sig []byte) error {
	response, ok := responseProtoData.(*pb.CustomProtocolRes)
	if !ok {
		return errors.New("CustomStreamProtocolAdapter->SetResponseSig: failed to cast response to *pb.CustomProtocolRes")
	}
	response.BasicData.Sig = sig
	return nil
}

func (adapter *CustomStreamProtocolAdapter) CallRequestCallback(requestProtoData protoreflect.ProtoMessage) (any, any, error) {
	data, retCode, err := adapter.protocol.Callback.OnCustomStreamProtocolRequest(requestProtoData)
	return data, retCode, err
}

func (adapter *CustomStreamProtocolAdapter) CallResponseCallback(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	data, err := adapter.protocol.Callback.OnCustomStreamProtocolResponse(requestProtoData, responseProtoData)
	return data, err
}

func NewCustomStreamProtocol(
	ctx context.Context,
	host host.Host,
	pid string,
	protocolService common.ProtocolService,
	protocolCallback common.StreamProtocolCallback) *common.StreamProtocol {
	adapter := NewCustomStreamProtocolAdapter()
	adapter.pid = pid
	protocol := common.NewStreamProtocol(ctx, host, protocolService, protocolCallback, adapter)
	adapter.protocol = protocol
	adapter.init()
	return adapter.protocol
}
