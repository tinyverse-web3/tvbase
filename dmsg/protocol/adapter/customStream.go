package adapter

import (
	"context"
	"errors"
	"fmt"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
	customProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol/custom"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type CustomStreamProtocolAdapter struct {
	CommonProtocolAdapter
	protocol *dmsgProtocol.CustomSProtocol
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
	requestProtoMsg := &pb.CustomProtocolReq{
		BasicData: basicData,
	}
	if len(dataList) == 2 {
		pid, ok := dataList[0].(string)
		if !ok {
			return nil, errors.New("CustomStreamProtocolAdapter->InitRequest: failed to cast datalist[0] to string for get customProtocolID")
		}
		content, ok := dataList[1].([]byte)
		if !ok {
			return nil, errors.New("CustomStreamProtocolAdapter->InitRequest: failed to cast datalist[1] to []byte for get content")
		}
		requestProtoMsg.PID = pid
		requestProtoMsg.Content = content
	} else {
		return requestProtoMsg, errors.New("CustomStreamProtocolAdapter->InitRequest: parameter dataList need contain customProtocolID and content")
	}
	return requestProtoMsg, nil
}

func (adapter *CustomStreamProtocolAdapter) InitResponse(
	requestProtoData protoreflect.ProtoMessage,
	basicData *pb.BasicData,
	dataList ...any) (protoreflect.ProtoMessage, error) {
	retCode := dmsgProtocol.NewSuccRetCode()
	if len(dataList) > 1 {
		var ok bool
		retCode, ok = dataList[1].(*pb.RetCode)
		if !ok {
			return nil, fmt.Errorf("CustomStreamProtocolAdapter->InitResponse: fail to cast dataList[1] to *pb.RetCode")
		}
	}
	response := &pb.CustomProtocolRes{
		BasicData: basicData,
		RetCode:   retCode,
	}

	request, ok := requestProtoData.(*pb.CustomProtocolReq)
	if !ok {
		return response, fmt.Errorf("CustomStreamProtocolAdapter->InitResponse: fail to cast requestProtoData to *pb.CustomContentReq")
	}

	if len(dataList) < 1 {
		return nil, errors.New("CustomStreamProtocolAdapter:InitResponse: dataList need contain customStreamProtocolResponseParam")
	}
	customStreamProtocolResponseParam, ok := dataList[0].(*customProtocol.ResponseParam)
	if !ok {
		return response, fmt.Errorf("CustomStreamProtocolAdapter->InitResponse: fail to cast dataList[0] to CustomStreamProtocolResponseParam")
	}

	response.PID = customStreamProtocolResponseParam.PID

	// get response.Content
	err := customStreamProtocolResponseParam.Service.HandleResponse(request, response)
	if err != nil {
		return response, err
	}

	return response, nil
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

func (adapter *CustomStreamProtocolAdapter) SetResponseRetCode(
	responseProtoMsg protoreflect.ProtoMessage,
	code int32,
	result string) {
	request, ok := responseProtoMsg.(*pb.CustomProtocolRes)
	if !ok {
		return
	}
	request.RetCode = dmsgProtocol.NewRetCode(code, result)
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
	requestProtoData protoreflect.ProtoMessage) (any, any, bool, error) {
	data, retCode, abort, err := adapter.protocol.Callback.OnCustomRequest(requestProtoData)
	return data, retCode, abort, err
}

func (adapter *CustomStreamProtocolAdapter) CallResponseCallback(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	data, err := adapter.protocol.Callback.OnCustomResponse(requestProtoData, responseProtoData)
	return data, err
}

func NewCustomStreamProtocol(
	ctx context.Context,
	host host.Host,
	pid string,
	callbck dmsgProtocol.CustomSpCallback,
	service dmsgProtocol.DmsgServiceInterface) *dmsgProtocol.CustomSProtocol {
	ret := NewCustomStreamProtocolAdapter()
	ret.pid = pid
	protocol := dmsgProtocol.NewCustomSProtocol(ctx, host, callbck, service, ret)
	ret.protocol = protocol
	return protocol
}
