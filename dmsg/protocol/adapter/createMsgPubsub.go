package adapter

import (
	"context"
	"errors"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"

	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/basic"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/common"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type CreateMsgPubsubProtocolAdapter struct {
	AbstructProtocolAdapter
	protocol *basic.CreatePubsubSProtocol
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
	return common.PidCreateMsgPubsubReq
}

func (adapter *CreateMsgPubsubProtocolAdapter) GetStreamResponsePID() protocol.ID {
	return common.PidCreateMsgPubsubRes
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
		pubkey, ok := dataList[0].(string)
		if !ok {
			return request, errors.New("CreatePubusubProtocolAdapter->InitRequest: failed to cast datalist[0] to string for key")
		}
		request.Key = pubkey
	} else {
		return request, errors.New("CreatePubusubProtocolAdapter->InitRequest: parameter dataList need contain key")
	}

	return request, nil
}

func (adapter *CreateMsgPubsubProtocolAdapter) InitResponse(
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
	callback common.CreatePubsubSpCallback,
	service common.DmsgService,
	enableRequest bool,
	pubkey string,
) *basic.CreatePubsubSProtocol {
	adapter := NewCreateMsgPubsubProtocolAdapter()
	protocol := basic.NewCreateMsgPubsubSProtocol(ctx, host, callback, service, adapter, enableRequest, pubkey)
	adapter.protocol = protocol
	return protocol
}
