package adapter

import (
	"context"
	"errors"
	"fmt"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"

	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type CreateMsgPubsubProtocolAdapter struct {
	CommonProtocolAdapter
	protocol *dmsgProtocol.CreatePubsubSProtocol
}

func NewCreateMsgPubsubProtocolAdapter() *CreateMsgPubsubProtocolAdapter {
	ret := &CreateMsgPubsubProtocolAdapter{}
	return ret
}

func (adapter *CreateMsgPubsubProtocolAdapter) GetRequestPID() pb.PID {
	return pb.PID_CREATE_MSG_PUBSUB_REQ
}

func (adapter *CreateMsgPubsubProtocolAdapter) GetResponsePID() pb.PID {
	return pb.PID_CREATE_MSG_PUBSUB_RES
}

func (adapter *CreateMsgPubsubProtocolAdapter) GetStreamRequestPID() protocol.ID {
	return dmsgProtocol.PidCreateMsgPubsubReq
}

func (adapter *CreateMsgPubsubProtocolAdapter) GetStreamResponsePID() protocol.ID {
	return dmsgProtocol.PidCreateMsgPubsubRes
}

func (adapter *CreateMsgPubsubProtocolAdapter) GetEmptyRequest() protoreflect.ProtoMessage {
	return &pb.CreatePubsubReq{}
}
func (adapter *CreateMsgPubsubProtocolAdapter) GetEmptyResponse() protoreflect.ProtoMessage {
	return &pb.CreatePubsubRes{}
}

func (adapter *CreateMsgPubsubProtocolAdapter) InitRequest(
	basicData *pb.BasicData,
	dataList ...any) (protoreflect.ProtoMessage, error) {
	request := &pb.CreatePubsubReq{
		BasicData: basicData,
	}

	if len(dataList) == 1 {
		key, ok := dataList[0].(string)
		if !ok {
			return request, errors.New("CreatePubusubProtocolAdapter->InitRequest: failed to cast datalist[0] to string for key")
		}
		request.Key = key
	} else {
		return request, errors.New("CreatePubusubProtocolAdapter->InitRequest: parameter dataList need contain key")
	}

	return request, nil
}

func (adapter *CreateMsgPubsubProtocolAdapter) InitResponse(
	requestProtoData protoreflect.ProtoMessage,
	basicData *pb.BasicData,
	dataList ...any) (protoreflect.ProtoMessage, error) {
	retCode := dmsgProtocol.NewSuccRetCode()
	if len(dataList) > 1 && dataList[1] != nil {
		var ok bool
		retCode, ok = dataList[1].(*pb.RetCode)
		if !ok {
			return nil, fmt.Errorf("CreatePubusubProtocolAdapter->InitResponse: fail to cast dataList[1] to *pb.RetCode")
		}
	}
	response := &pb.CreatePubsubRes{
		BasicData: basicData,
		RetCode:   retCode,
	}

	return response, nil
}

func (adapter *CreateMsgPubsubProtocolAdapter) GetRequestBasicData(
	requestProtoMsg protoreflect.ProtoMessage) *pb.BasicData {
	request, ok := requestProtoMsg.(*pb.CreatePubsubReq)
	if !ok {
		return nil
	}
	return request.BasicData
}

func (adapter *CreateMsgPubsubProtocolAdapter) GetResponseBasicData(
	responseProtoMsg protoreflect.ProtoMessage) *pb.BasicData {
	response, ok := responseProtoMsg.(*pb.CreatePubsubRes)
	if !ok {
		return nil
	}
	return response.BasicData
}

func (adapter *CreateMsgPubsubProtocolAdapter) GetResponseRetCode(
	responseProtoMsg protoreflect.ProtoMessage) *pb.RetCode {
	response, ok := responseProtoMsg.(*pb.CreatePubsubRes)
	if !ok {
		return nil
	}
	return response.RetCode
}

func (adapter *CreateMsgPubsubProtocolAdapter) SetRequestSig(
	requestProtoMsg protoreflect.ProtoMessage,
	sig []byte) error {
	request, ok := requestProtoMsg.(*pb.CreatePubsubReq)
	if !ok {
		return fmt.Errorf("CreatePubusubProtocolAdapter->SetRequestSig: failed to cast request to *pb.CreatePubsubReq")
	}
	request.BasicData.Sig = sig
	return nil
}

func (adapter *CreateMsgPubsubProtocolAdapter) SetResponseSig(
	responseProtoMsg protoreflect.ProtoMessage,
	sig []byte) error {
	response, ok := responseProtoMsg.(*pb.CreatePubsubRes)
	if !ok {
		return fmt.Errorf("CreatePubusubProtocolAdapter->SetResponseSig: failed to cast request to *pb.CreatePubsubRes")
	}
	response.BasicData.Sig = sig
	return nil
}

func (adapter *CreateMsgPubsubProtocolAdapter) SetResponseRetCode(
	responseProtoMsg protoreflect.ProtoMessage,
	code int32,
	result string) {
	request, ok := responseProtoMsg.(*pb.CreatePubsubRes)
	if !ok {
		return
	}
	request.RetCode = dmsgProtocol.NewRetCode(code, result)
}

func (adapter *CreateMsgPubsubProtocolAdapter) CallRequestCallback(
	requestProtoData protoreflect.ProtoMessage) (any, any, bool, error) {
	data, retCode, abort, err := adapter.protocol.Callback.OnCreatePubsubRequest(requestProtoData)
	return data, retCode, abort, err
}

func (adapter *CreateMsgPubsubProtocolAdapter) CallResponseCallback(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	data, err := adapter.protocol.Callback.OnCreatePubsubResponse(requestProtoData, responseProtoData)
	return data, err
}

func NewCreateMsgPubsubProtocol(
	ctx context.Context,
	host host.Host,
	callback dmsgProtocol.CreatePubsubSpCallback,
	service dmsgProtocol.DmsgServiceInterface,
	enableRequest bool,
) *dmsgProtocol.CreatePubsubSProtocol {
	adapter := NewCreateMsgPubsubProtocolAdapter()
	protocol := dmsgProtocol.NewCreateMsgPubsubSProtocol(ctx, host, callback, service, adapter, enableRequest)
	adapter.protocol = protocol
	return protocol
}
