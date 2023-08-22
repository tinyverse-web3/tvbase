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

type CreateChannelProtocolAdapter struct {
	CommonProtocolAdapter
	protocol *dmsgProtocol.CreatePubsubSProtocol
}

func NewCreateChannelProtocolAdapter() *CreateChannelProtocolAdapter {
	ret := &CreateChannelProtocolAdapter{}
	return ret
}

func (adapter *CreateChannelProtocolAdapter) GetRequestPID() pb.PID {
	return pb.PID_CREATE_CHANNEL_REQ
}

func (adapter *CreateChannelProtocolAdapter) GetResponsePID() pb.PID {
	return pb.PID_CREATE_CHANNEL_RES
}

func (adapter *CreateChannelProtocolAdapter) GetStreamRequestPID() protocol.ID {
	return dmsgProtocol.PidCreateChannelReq
}

func (adapter *CreateChannelProtocolAdapter) GetStreamResponsePID() protocol.ID {
	return dmsgProtocol.PidCreateChannelRes
}

func (adapter *CreateChannelProtocolAdapter) GetEmptyRequest() protoreflect.ProtoMessage {
	return &pb.CreatePubsubReq{}
}
func (adapter *CreateChannelProtocolAdapter) GetEmptyResponse() protoreflect.ProtoMessage {
	return &pb.CreatePubsubRes{}
}

func (adapter *CreateChannelProtocolAdapter) InitRequest(
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

func (adapter *CreateChannelProtocolAdapter) InitResponse(
	requestProtoData protoreflect.ProtoMessage,
	basicData *pb.BasicData,
	dataList ...any) (protoreflect.ProtoMessage, error) {
	retCode, err := GetRetCode(dataList)
	if err != nil {
		return nil, err
	}
	response := &pb.CreatePubsubRes{
		BasicData: basicData,
		RetCode:   retCode,
	}

	return response, nil
}

func (adapter *CreateChannelProtocolAdapter) GetRequestBasicData(
	requestProtoMsg protoreflect.ProtoMessage) *pb.BasicData {
	request, ok := requestProtoMsg.(*pb.CreatePubsubReq)
	if !ok {
		return nil
	}
	return request.BasicData
}

func (adapter *CreateChannelProtocolAdapter) GetResponseBasicData(
	responseProtoMsg protoreflect.ProtoMessage) *pb.BasicData {
	response, ok := responseProtoMsg.(*pb.CreatePubsubRes)
	if !ok {
		return nil
	}
	return response.BasicData
}

func (adapter *CreateChannelProtocolAdapter) GetResponseRetCode(
	responseProtoMsg protoreflect.ProtoMessage) *pb.RetCode {
	response, ok := responseProtoMsg.(*pb.CreatePubsubRes)
	if !ok {
		return nil
	}
	return response.RetCode
}

func (adapter *CreateChannelProtocolAdapter) SetRequestSig(
	requestProtoMsg protoreflect.ProtoMessage,
	sig []byte) error {
	request, ok := requestProtoMsg.(*pb.CreatePubsubReq)
	if !ok {
		return fmt.Errorf("CreatePubusubProtocolAdapter->SetRequestSig: failed to cast request to *pb.CreatePubsubReq")
	}
	request.BasicData.Sig = sig
	return nil
}

func (adapter *CreateChannelProtocolAdapter) SetResponseSig(
	responseProtoMsg protoreflect.ProtoMessage,
	sig []byte) error {
	response, ok := responseProtoMsg.(*pb.CreatePubsubRes)
	if !ok {
		return fmt.Errorf("CreatePubusubProtocolAdapter->SetResponseSig: failed to cast request to *pb.CreatePubsubRes")
	}
	response.BasicData.Sig = sig
	return nil
}

func (adapter *CreateChannelProtocolAdapter) SetResponseRetCode(
	responseProtoMsg protoreflect.ProtoMessage,
	code int32,
	result string) {
	request, ok := responseProtoMsg.(*pb.CreatePubsubRes)
	if !ok {
		return
	}
	request.RetCode = dmsgProtocol.NewRetCode(code, result)
}

func (adapter *CreateChannelProtocolAdapter) CallRequestCallback(
	requestProtoData protoreflect.ProtoMessage) (any, any, bool, error) {
	data, retCode, abort, err := adapter.protocol.Callback.OnCreatePubsubRequest(requestProtoData)
	return data, retCode, abort, err
}

func (adapter *CreateChannelProtocolAdapter) CallResponseCallback(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	data, err := adapter.protocol.Callback.OnCreatePubsubResponse(requestProtoData, responseProtoData)
	return data, err
}

func NewCreateChannelProtocol(
	ctx context.Context,
	host host.Host,
	callback dmsgProtocol.CreatePubsubSpCallback,
	service dmsgProtocol.DmsgServiceInterface,
	enableRequest bool,
) *dmsgProtocol.CreatePubsubSProtocol {
	adapter := NewCreateChannelProtocolAdapter()
	protocol := dmsgProtocol.NewCreateChannelSProtocol(ctx, host, callback, service, adapter, enableRequest)
	adapter.protocol = protocol
	return protocol
}
