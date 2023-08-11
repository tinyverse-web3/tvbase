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
	protocol *dmsgProtocol.StreamProtocol
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
	return &pb.CreateChannelReq{}
}
func (adapter *CreateChannelProtocolAdapter) GetEmptyResponse() protoreflect.ProtoMessage {
	return &pb.CreateChannelRes{}
}

func (adapter *CreateChannelProtocolAdapter) InitRequest(
	basicData *pb.BasicData,
	dataList ...any) (protoreflect.ProtoMessage, error) {
	requestProtoMsg := &pb.CreateChannelReq{
		BasicData: basicData,
	}

	if len(dataList) == 1 {
		channelKey, ok := dataList[0].(string)
		if !ok {
			return requestProtoMsg, errors.New("CreateChannelProtocolAdapter->InitRequest: failed to cast datalist[0] to string for channelKey")
		}
		requestProtoMsg.ChannelKey = channelKey
	} else {
		return requestProtoMsg, errors.New("CreateChannelProtocolAdapter->InitRequest: parameter dataList need contain channelKey")
	}

	return requestProtoMsg, nil
}

func (adapter *CreateChannelProtocolAdapter) InitResponse(
	basicData *pb.BasicData,
	dataList ...any) (protoreflect.ProtoMessage, error) {
	responseProtoMsg := &pb.CreateChannelRes{
		BasicData: basicData,
		RetCode:   dmsgProtocol.NewSuccRetCode(),
	}
	return responseProtoMsg, nil
}

func (adapter *CreateChannelProtocolAdapter) GetRequestBasicData(
	requestProtoMsg protoreflect.ProtoMessage) *pb.BasicData {
	request, ok := requestProtoMsg.(*pb.CreateChannelReq)
	if !ok {
		return nil
	}
	return request.BasicData
}

func (adapter *CreateChannelProtocolAdapter) GetResponseBasicData(
	responseProtoMsg protoreflect.ProtoMessage) *pb.BasicData {
	response, ok := responseProtoMsg.(*pb.CreateChannelRes)
	if !ok {
		return nil
	}
	return response.BasicData
}

func (adapter *CreateChannelProtocolAdapter) GetResponseRetCode(
	responseProtoMsg protoreflect.ProtoMessage) *pb.RetCode {
	response, ok := responseProtoMsg.(*pb.CreateChannelRes)
	if !ok {
		return nil
	}
	return response.RetCode
}

func (adapter *CreateChannelProtocolAdapter) SetRequestSig(
	requestProtoMsg protoreflect.ProtoMessage,
	sig []byte) error {
	request, ok := requestProtoMsg.(*pb.CreateChannelReq)
	if !ok {
		return fmt.Errorf("CreateChannelProtocolAdapter->SetRequestSig: failed to cast request to *pb.CreateChannelReq")
	}
	request.BasicData.Sig = sig
	return nil
}

func (adapter *CreateChannelProtocolAdapter) SetResponseSig(
	responseProtoMsg protoreflect.ProtoMessage,
	sig []byte) error {
	response, ok := responseProtoMsg.(*pb.CreateChannelRes)
	if !ok {
		return fmt.Errorf("CreateChannelProtocolAdapter->SetResponseSig: failed to cast request to *pb.CreateChannelRes")
	}
	response.BasicData.Sig = sig
	return nil
}

func (adapter *CreateChannelProtocolAdapter) SetResponseRetCode(
	responseProtoMsg protoreflect.ProtoMessage,
	code int32,
	result string) {
	request, ok := responseProtoMsg.(*pb.CreateChannelRes)
	if !ok {
		return
	}
	request.RetCode = dmsgProtocol.NewRetCode(code, result)
}

func (adapter *CreateChannelProtocolAdapter) CallRequestCallback(
	requestProtoData protoreflect.ProtoMessage) (interface{}, error) {
	data, err := adapter.protocol.Callback.OnCreateChannelRequest(requestProtoData)
	return data, err
}

func (adapter *CreateChannelProtocolAdapter) CallResponseCallback(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (interface{}, error) {
	data, err := adapter.protocol.Callback.OnCreateChannelResponse(requestProtoData, responseProtoData)
	return data, err
}

func NewCreateChannelProtocol(
	ctx context.Context,
	host host.Host,
	callback dmsgProtocol.StreamProtocolCallback,
	service dmsgProtocol.ProtocolService) *dmsgProtocol.StreamProtocol {
	adapter := NewCreateChannelProtocolAdapter()
	protocol := dmsgProtocol.NewStreamProtocol(ctx, host, callback, service, adapter)
	adapter.protocol = protocol
	return protocol
}
