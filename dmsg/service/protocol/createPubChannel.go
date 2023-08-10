package protocol

import (
	"context"
	"errors"

	"github.com/libp2p/go-libp2p/core/host"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/service/common"
)

type CreateChannelProtocolAdapter struct {
	common.CommonProtocolAdapter
	protocol *common.StreamProtocol
}

func NewCreateChannelProtocolAdapter() *CreateChannelProtocolAdapter {
	ret := &CreateChannelProtocolAdapter{}
	return ret
}

func (adapter *CreateChannelProtocolAdapter) init() {
}

func (adapter *CreateChannelProtocolAdapter) GetRequestPID() pb.PID {
	return pb.PID_CREATE_CHANNEL_REQ
}

func (adapter *CreateChannelProtocolAdapter) GetResponsePID() pb.PID {
	return pb.PID_CREATE_CHANNEL_RES
}

func (adapter *CreateChannelProtocolAdapter) GetStreamResponsePID() protocol.ID {
	return dmsgProtocol.PidCreateChannelRes
}

func (adapter *CreateChannelProtocolAdapter) GetStreamRequestPID() protocol.ID {
	return dmsgProtocol.PidCreateChannelReq
}

func (adapter *CreateChannelProtocolAdapter) GetEmptyRequest() protoreflect.ProtoMessage {
	return &pb.CreateChannelReq{}
}
func (adapter *CreateChannelProtocolAdapter) GetEmptyResponse() protoreflect.ProtoMessage {
	return &pb.CreateChannelRes{}
}

func (adapter *CreateChannelProtocolAdapter) GetRequestBasicData(requestProtoData protoreflect.ProtoMessage) *pb.BasicData {
	request, ok := requestProtoData.(*pb.CreateChannelReq)
	if !ok {
		return nil
	}
	return request.BasicData
}

func (adapter *CreateChannelProtocolAdapter) GetResponseBasicData(responseProtoData protoreflect.ProtoMessage) *pb.BasicData {
	request, ok := responseProtoData.(*pb.CreateChannelRes)
	if !ok {
		return nil
	}
	return request.BasicData
}

func (adapter *CreateChannelProtocolAdapter) InitResponse(
	requestProtoData protoreflect.ProtoMessage,
	basicData *pb.BasicData,
	dataList ...any) (protoreflect.ProtoMessage, error) {
	response := &pb.CreateChannelRes{
		BasicData: basicData,
		RetCode:   dmsgProtocol.NewSuccRetCode(),
	}
	return response, nil
}

func (adapter *CreateChannelProtocolAdapter) SetResponseSig(responseProtoData protoreflect.ProtoMessage, sig []byte) error {
	response, ok := responseProtoData.(*pb.CreateChannelRes)
	if !ok {
		return errors.New("ChannelProtocolAdapter.SetResponseSig: failed to cast response to *pb.CreateChannelRes")
	}
	response.BasicData.Sig = sig
	return nil
}

func (adapter *CreateChannelProtocolAdapter) CallRequestCallback(requestProtoData protoreflect.ProtoMessage) (any, any, error) {
	data, retCode, err := adapter.protocol.Callback.OnCreateChannelRequest(requestProtoData)
	return data, retCode, err
}

func NewCreateChannelProtocol(ctx context.Context, host host.Host, protocolService common.ProtocolService, protocolCallback common.StreamProtocolCallback) *common.StreamProtocol {
	adapter := NewCreateChannelProtocolAdapter()
	protocol := common.NewStreamProtocol(ctx, host, protocolService, protocolCallback, adapter)
	adapter.protocol = protocol
	adapter.init()
	return protocol
}
