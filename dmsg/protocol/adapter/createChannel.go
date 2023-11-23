package adapter

import (
	"context"
	"errors"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"

	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/basic"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/common"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/newProtocol"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type CreateChannelProtocolAdapter struct {
	AbstructProtocolAdapter
	protocol *basic.CreatePubsubSProtocol
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
	return common.PidCreateChannelReq
}

func (adapter *CreateChannelProtocolAdapter) GetStreamResponsePID() protocol.ID {
	return common.PidCreateChannelRes
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
	retCode, err := getRetCode(dataList)
	if err != nil {
		return nil, err
	}
	response := &pb.CreatePubsubRes{
		BasicData: basicData,
		RetCode:   retCode,
	}

	return response, nil
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
	callback common.CreatePubsubSpCallback,
	service common.DmsgService,
	enableRequest bool,
	pubkey string,
) *basic.CreatePubsubSProtocol {
	adapter := NewCreateChannelProtocolAdapter()
	protocol := newProtocol.NewCreateChannelSProtocol(ctx, host, callback, service, adapter, enableRequest, pubkey)
	adapter.protocol = protocol
	return protocol
}
