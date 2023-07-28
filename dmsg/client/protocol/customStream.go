package protocol

import (
	"context"
	"errors"
	"fmt"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/client/common"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
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

func (adapter *CustomStreamProtocolAdapter) GetResponsePID() pb.PID {
	return pb.PID_CUSTOM_STREAM_RES
}

func (adapter *CustomStreamProtocolAdapter) GetRequestPID() pb.PID {
	return pb.PID_CUSTOM_STREAM_REQ
}

func (adapter *CustomStreamProtocolAdapter) GetStreamRequestPID() protocol.ID {
	return protocol.ID(dmsgProtocol.PidCustomProtocolReq + "/" + adapter.pid)
}

func (adapter *CustomStreamProtocolAdapter) GetStreamResponsePID() protocol.ID {
	return protocol.ID(dmsgProtocol.PidCustomProtocolRes + "/" + adapter.pid)
}

func (adapter *CustomStreamProtocolAdapter) GetEmptyRequest() protoreflect.ProtoMessage {
	return &pb.CustomProtocolReq{}
}
func (adapter *CustomStreamProtocolAdapter) GetEmptyResponse() protoreflect.ProtoMessage {
	return &pb.CustomProtocolRes{}
}

func (adapter *CustomStreamProtocolAdapter) InitRequest(
	basicData *pb.BasicData,
	dataList ...any) (protoreflect.ProtoMessage, error) {
	var requestProtoMsg *pb.CustomProtocolReq
	if len(dataList) == 2 {
		pid, ok := dataList[0].(string)
		if !ok {
			return nil, errors.New("CustomStreamProtocolAdapter->InitRequest: failed to cast datalist[0] to string for get customProtocolID")
		}
		content, ok := dataList[1].([]byte)
		if !ok {
			return nil, errors.New("CustomStreamProtocolAdapter->InitRequest: failed to cast datalist[1] to []byte for get content")
		}
		requestProtoMsg = &pb.CustomProtocolReq{
			BasicData: basicData,
			PID:       pid,
			Content:   content,
		}
	} else {
		return nil, errors.New("CustomStreamProtocolAdapter->InitRequest: parameter dataList need contain customProtocolID and content")
	}
	return requestProtoMsg, nil
}

func (adapter *CustomStreamProtocolAdapter) InitResponse(
	basicData *pb.BasicData,
	dataList ...any) (protoreflect.ProtoMessage, error) {
	var responseProtoMsg *pb.CustomProtocolRes
	if len(dataList) == 2 {
		pid, ok := dataList[0].(string)
		if !ok {
			return nil, errors.New("CustomStreamProtocolAdapter->InitResponse: failed to cast datalist[0] to string for get customProtocolID")
		}
		content, ok := dataList[1].([]byte)
		if !ok {
			return nil, errors.New("CustomStreamProtocolAdapter->InitResponse: failed to cast datalist[1] to []byte for get content")
		}
		responseProtoMsg = &pb.CustomProtocolRes{
			BasicData: basicData,
			PID:       pid,
			Content:   content,
			RetCode:   dmsgProtocol.NewSuccRetCode(),
		}
	} else {
		return nil, errors.New("CustomStreamProtocolAdapter->InitResponse: parameter dataList need contain customProtocolID and content")
	}
	return responseProtoMsg, nil
}

func (adapter *CustomStreamProtocolAdapter) GetRequestBasicData(
	requestProtoMsg protoreflect.ProtoMessage) *pb.BasicData {
	request, ok := requestProtoMsg.(*pb.CustomProtocolReq)
	if !ok {
		return nil
	}
	return request.BasicData
}

func (adapter *CustomStreamProtocolAdapter) GetResponseBasicData(
	responseProtoMsg protoreflect.ProtoMessage) *pb.BasicData {
	response, ok := responseProtoMsg.(*pb.CustomProtocolRes)
	if !ok {
		return nil
	}
	return response.BasicData
}

func (adapter *CustomStreamProtocolAdapter) GetResponseRetCode(
	responseProtoMsg protoreflect.ProtoMessage) *pb.RetCode {
	response, ok := responseProtoMsg.(*pb.CustomProtocolRes)
	if !ok {
		return nil
	}
	return response.RetCode
}

func (adapter *CustomStreamProtocolAdapter) SetRequestSig(
	requestProtoMsg protoreflect.ProtoMessage,
	sig []byte) error {
	request, ok := requestProtoMsg.(*pb.CustomProtocolReq)
	if !ok {
		return fmt.Errorf("CustomStreamProtocolAdapter->SetRequestSig: failed to cast request to *pb.CustomProtocolReq")
	}
	request.BasicData.Sig = sig
	return nil
}

func (adapter *CustomStreamProtocolAdapter) SetResponseSig(
	responseProtoMsg protoreflect.ProtoMessage, sig []byte) error {
	response, ok := responseProtoMsg.(*pb.CustomProtocolRes)
	if !ok {
		return fmt.Errorf("CustomStreamProtocolAdapter->SetResponseSig: failed to cast request to *pb.CustomProtocolRes")
	}
	response.BasicData.Sig = sig
	return nil
}

func (adapter *CustomStreamProtocolAdapter) CallRequestCallback(
	requestProtoData protoreflect.ProtoMessage) (interface{}, error) {
	data, err := adapter.protocol.Callback.OnCustomStreamProtocolRequest(requestProtoData)
	return data, err
}

func (adapter *CustomStreamProtocolAdapter) CallResponseCallback(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (interface{}, error) {
	data, err := adapter.protocol.Callback.OnCustomStreamProtocolResponse(requestProtoData, responseProtoData)
	return data, err
}

func NewCustomStreamProtocol(
	ctx context.Context,
	host host.Host,
	customProtocolId string,
	protocolCallback common.StreamProtocolCallback,
	protocolService common.ProtocolService,
) *common.StreamProtocol {
	ret := NewCustomStreamProtocolAdapter()
	ret.pid = customProtocolId
	protocol := common.NewStreamProtocol(ctx, host, protocolCallback, protocolService, ret)
	ret.protocol = protocol
	return protocol
}
