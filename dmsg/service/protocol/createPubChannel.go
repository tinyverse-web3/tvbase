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

type PubChannelProtocolAdapter struct {
	common.CommonProtocolAdapter
	protocol *common.StreamProtocol
}

func NewCreatePubChannelProtocolAdapter() *PubChannelProtocolAdapter {
	ret := &PubChannelProtocolAdapter{}
	return ret
}

func (adapter *PubChannelProtocolAdapter) init() {
}

func (adapter *PubChannelProtocolAdapter) GetRequestPID() pb.PID {
	return pb.PID_CREATE_PUB_CHANNEL_REQ
}

func (adapter *PubChannelProtocolAdapter) GetResponsePID() pb.PID {
	return pb.PID_CREATE_PUB_CHANNEL_RES
}

func (adapter *PubChannelProtocolAdapter) GetStreamResponsePID() protocol.ID {
	return dmsgProtocol.PidCreatePubChannelRes
}

func (adapter *PubChannelProtocolAdapter) GetStreamRequestPID() protocol.ID {
	return dmsgProtocol.PidCreatePubChannelReq
}

func (adapter *PubChannelProtocolAdapter) GetEmptyRequest() protoreflect.ProtoMessage {
	return &pb.CreatePubChannelReq{}
}
func (adapter *PubChannelProtocolAdapter) GetEmptyResponse() protoreflect.ProtoMessage {
	return &pb.CreatePubChannelRes{}
}

func (adapter *PubChannelProtocolAdapter) GetRequestBasicData(requestProtoData protoreflect.ProtoMessage) *pb.BasicData {
	request, ok := requestProtoData.(*pb.CreatePubChannelReq)
	if !ok {
		return nil
	}
	return request.BasicData
}

func (adapter *PubChannelProtocolAdapter) GetResponseBasicData(responseProtoData protoreflect.ProtoMessage) *pb.BasicData {
	request, ok := responseProtoData.(*pb.CreatePubChannelRes)
	if !ok {
		return nil
	}
	return request.BasicData
}

func (adapter *PubChannelProtocolAdapter) InitResponse(
	requestProtoData protoreflect.ProtoMessage,
	basicData *pb.BasicData,
	dataList ...any) (protoreflect.ProtoMessage, error) {
	response := &pb.CreatePubChannelRes{
		BasicData: basicData,
		RetCode:   dmsgProtocol.NewSuccRetCode(),
	}
	return response, nil
}

func (adapter *PubChannelProtocolAdapter) SetResponseSig(responseProtoData protoreflect.ProtoMessage, sig []byte) error {
	response, ok := responseProtoData.(*pb.CreatePubChannelRes)
	if !ok {
		return errors.New("PubChannelProtocolAdapter.SetResponseSig: failed to cast response to *pb.CreatePubChannelRes")
	}
	response.BasicData.Sig = sig
	return nil
}

func (adapter *PubChannelProtocolAdapter) CallRequestCallback(requestProtoData protoreflect.ProtoMessage) (any, any, error) {
	data, retCode, err := adapter.protocol.Callback.OnCreatePubChannelRequest(requestProtoData)
	return data, retCode, err
}

func NewCreatePubChannelProtocol(ctx context.Context, host host.Host, protocolService common.ProtocolService, protocolCallback common.StreamProtocolCallback) *common.StreamProtocol {
	adapter := NewCreatePubChannelProtocolAdapter()
	protocol := common.NewStreamProtocol(ctx, host, protocolService, protocolCallback, adapter)
	adapter.protocol = protocol
	adapter.init()
	return protocol
}
