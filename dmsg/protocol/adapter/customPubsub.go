package adapter

import (
	"context"
	"errors"
	"fmt"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
	customProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol/custom"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type CustomStreamProtocolResponseParam struct {
	PID     string
	Service customProtocol.CustomStreamProtocolService
}

type CustomPubsubProtocolAdapter struct {
	CommonProtocolAdapter
	protocol *dmsgProtocol.PubsubProtocol
}

func NewCustomPubsubProtocolAdapter() *CustomPubsubProtocolAdapter {
	ret := &CustomPubsubProtocolAdapter{}
	return ret
}

func (adapter *CustomPubsubProtocolAdapter) GetRequestPID() pb.PID {
	return pb.PID_CUSTOM_PUBSUB_REQ
}

func (adapter *CustomPubsubProtocolAdapter) GetResponsePID() pb.PID {
	return pb.PID_CUSTOM_PUBSUB_RES
}

func (adapter *CustomPubsubProtocolAdapter) GetEmptyRequest() protoreflect.ProtoMessage {
	return &pb.CustomProtocolReq{}
}
func (adapter *CustomPubsubProtocolAdapter) GetEmptyResponse() protoreflect.ProtoMessage {
	return &pb.CustomProtocolRes{}
}

func (adapter *CustomPubsubProtocolAdapter) InitRequest(
	basicData *pb.BasicData,
	dataList ...any) (protoreflect.ProtoMessage, error) {
	requestProtoMsg := &pb.CustomProtocolReq{
		BasicData: basicData,
	}
	if len(dataList) == 2 {
		pid, ok := dataList[0].(string)
		if !ok {
			return nil, errors.New("CustomPubsubProtocolAdapter->InitRequest: failed to cast datalist[0] to string for get customProtocolID")
		}
		content, ok := dataList[1].([]byte)
		if !ok {
			return nil, errors.New("CustomPubsubProtocolAdapter->InitRequest: failed to cast datalist[1] to []byte for get content")
		}
		requestProtoMsg.PID = pid
		requestProtoMsg.Content = content
	} else {
		return requestProtoMsg, errors.New("CustomPubsubProtocolAdapter->InitRequest: parameter dataList need contain customProtocolID and content")
	}
	return requestProtoMsg, nil
}

func (adapter *CustomPubsubProtocolAdapter) InitResponse(
	requestProtoData protoreflect.ProtoMessage,
	basicData *pb.BasicData,
	dataList ...any) (protoreflect.ProtoMessage, error) {
	var retCode *pb.RetCode
	if len(dataList) > 1 {
		var ok bool
		retCode, ok = dataList[1].(*pb.RetCode)
		if !ok {
			retCode = dmsgProtocol.NewSuccRetCode()
		}
	}
	response := &pb.CustomProtocolRes{
		BasicData: basicData,
		RetCode:   retCode,
	}

	request, ok := requestProtoData.(*pb.CustomProtocolReq)
	if !ok {
		return response, fmt.Errorf("CustomPubsubProtocolAdapter->InitResponse: fail to cast requestProtoData to *pb.CustomContentReq")
	}

	if len(dataList) < 1 {
		return nil, errors.New("CustomPubsubProtocolAdapter:InitResponse: dataList need contain customStreamProtocolResponseParam")
	}
	customStreamProtocolResponseParam, ok := dataList[0].(*CustomStreamProtocolResponseParam)
	if !ok {
		return response, fmt.Errorf("CustomPubsubProtocolAdapter->InitResponse: fail to cast dataList[0] to CustomStreamProtocolResponseParam")
	}

	response.PID = customStreamProtocolResponseParam.PID

	// get response.Content
	err := customStreamProtocolResponseParam.Service.HandleResponse(request, response)
	if err != nil {
		return response, err
	}

	return response, nil
}

func (adapter *CustomPubsubProtocolAdapter) GetRequestBasicData(
	requestProtoMsg protoreflect.ProtoMessage) *pb.BasicData {
	request, ok := requestProtoMsg.(*pb.CustomProtocolReq)
	if !ok {
		return nil
	}
	return request.BasicData
}

func (adapter *CustomPubsubProtocolAdapter) GetResponseBasicData(
	responseProtoMsg protoreflect.ProtoMessage) *pb.BasicData {
	response, ok := responseProtoMsg.(*pb.CustomProtocolRes)
	if !ok {
		return nil
	}
	return response.BasicData
}

func (adapter *CustomPubsubProtocolAdapter) GetResponseRetCode(
	responseProtoMsg protoreflect.ProtoMessage) *pb.RetCode {
	response, ok := responseProtoMsg.(*pb.CustomProtocolRes)
	if !ok {
		return nil
	}
	return response.RetCode
}

func (adapter *CustomPubsubProtocolAdapter) SetResponseRetCode(
	responseProtoMsg protoreflect.ProtoMessage,
	code int32,
	result string) {
	request, ok := responseProtoMsg.(*pb.CustomProtocolRes)
	if !ok {
		return
	}
	request.RetCode = dmsgProtocol.NewRetCode(code, result)
}

func (adapter *CustomPubsubProtocolAdapter) SetRequestSig(
	requestProtoMsg protoreflect.ProtoMessage,
	sig []byte) error {
	request, ok := requestProtoMsg.(*pb.CustomProtocolReq)
	if !ok {
		return fmt.Errorf("CustomPubsubProtocolAdapter->SetRequestSig: failed to cast request to *pb.CustomProtocolReq")
	}
	request.BasicData.Sig = sig
	return nil
}

func (adapter *CustomPubsubProtocolAdapter) SetResponseSig(
	responseProtoMsg protoreflect.ProtoMessage,
	sig []byte) error {
	response, ok := responseProtoMsg.(*pb.CustomProtocolRes)
	if !ok {
		return fmt.Errorf("CreateMailboxProtocolAdapter->SetResponseSig: failed to cast request to *pb.CreateMailboxReq")
	}
	response.BasicData.Sig = sig
	return nil
}

func (adapter *CustomPubsubProtocolAdapter) CallRequestCallback(
	requestProtoData protoreflect.ProtoMessage) (any, any, error) {
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
	callback dmsgProtocol.PubsubProtocolCallback,
	service dmsgProtocol.ProtocolService) *dmsgProtocol.PubsubProtocol {
	adapter := NewCustomPubsubProtocolAdapter()
	protocol := dmsgProtocol.NewPubsubProtocol(ctx, host, callback, service, adapter)
	adapter.protocol = protocol
	return protocol
}
