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
	protocol *dmsgProtocol.MsgSProtocol
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
	request := &pb.CreateChannelReq{
		BasicData: basicData,
	}

	if len(dataList) == 1 {
		channelKey, ok := dataList[0].(string)
		if !ok {
			return request, errors.New("CreateChannelProtocolAdapter->InitRequest: failed to cast datalist[0] to string for channelKey")
		}
		request.ChannelKey = channelKey
	} else {
		return request, errors.New("CreateChannelProtocolAdapter->InitRequest: parameter dataList need contain channelKey")
	}

	return request, nil
}

func (adapter *CreateChannelProtocolAdapter) InitResponse(
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
	response := &pb.CreateChannelRes{
		BasicData: basicData,
		RetCode:   retCode,
	}

	return response, nil
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
	requestProtoData protoreflect.ProtoMessage) (any, any, error) {
	data, retCode, err := adapter.protocol.Callback.OnCreateChannelRequest(requestProtoData)
	return data, retCode, err
}

func (adapter *CreateChannelProtocolAdapter) CallResponseCallback(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	data, err := adapter.protocol.Callback.OnCreateChannelResponse(requestProtoData, responseProtoData)
	return data, err
}

func NewCreateChannelProtocol(
	ctx context.Context,
	host host.Host,
	callback dmsgProtocol.MsgSpCallback,
	service dmsgProtocol.DmsgServiceInterface) *dmsgProtocol.MsgSProtocol {
	adapter := NewCreateChannelProtocolAdapter()
	protocol := dmsgProtocol.NewMsgSProtocol(ctx, host, callback, service, adapter)
	adapter.protocol = protocol
	return protocol
}
