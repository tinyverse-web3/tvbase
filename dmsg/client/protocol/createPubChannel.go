package protocol

import (
	"context"
	"errors"
	"fmt"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/client/common"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type CreatePubChannelProtocolAdapter struct {
	common.CommonProtocolAdapter
	protocol *common.StreamProtocol
}

func NewCreatePubChannelProtocolAdapter() *CreatePubChannelProtocolAdapter {
	ret := &CreatePubChannelProtocolAdapter{}
	return ret
}

func (adapter *CreatePubChannelProtocolAdapter) GetRequestPID() pb.PID {
	return pb.PID_CREATE_PUB_CHANNEL_REQ
}

func (adapter *CreatePubChannelProtocolAdapter) GetResponsePID() pb.PID {
	return pb.PID_CREATE_PUB_CHANNEL_RES
}

func (adapter *CreatePubChannelProtocolAdapter) GetStreamRequestPID() protocol.ID {
	return dmsgProtocol.PidCreatePubChannelReq
}

func (adapter *CreatePubChannelProtocolAdapter) GetStreamResponsePID() protocol.ID {
	return dmsgProtocol.PidCreatePubChannelRes
}

func (adapter *CreatePubChannelProtocolAdapter) GetEmptyRequest() protoreflect.ProtoMessage {
	return &pb.CreatePubChannelReq{}
}
func (adapter *CreatePubChannelProtocolAdapter) GetEmptyResponse() protoreflect.ProtoMessage {
	return &pb.CreatePubChannelRes{}
}

func (adapter *CreatePubChannelProtocolAdapter) InitRequest(
	basicData *pb.BasicData,
	dataList ...any) (protoreflect.ProtoMessage, error) {
	requestProtoMsg := &pb.CreatePubChannelReq{
		BasicData: basicData,
	}

	if len(dataList) == 1 {
		channelKey, ok := dataList[0].(string)
		if !ok {
			return requestProtoMsg, errors.New("CreatePubChannelProtocolAdapter->InitRequest: failed to cast datalist[0] to string for channelKey")
		}
		requestProtoMsg.ChannelKey = channelKey
	} else {
		return requestProtoMsg, errors.New("CreatePubChannelProtocolAdapter->InitRequest: parameter dataList need contain channelKey")
	}

	return requestProtoMsg, nil
}

func (adapter *CreatePubChannelProtocolAdapter) InitResponse(
	basicData *pb.BasicData,
	dataList ...any) (protoreflect.ProtoMessage, error) {
	responseProtoMsg := &pb.CreatePubChannelRes{
		BasicData: basicData,
		RetCode:   dmsgProtocol.NewSuccRetCode(),
	}
	return responseProtoMsg, nil
}

func (adapter *CreatePubChannelProtocolAdapter) GetRequestBasicData(
	requestProtoMsg protoreflect.ProtoMessage) *pb.BasicData {
	request, ok := requestProtoMsg.(*pb.CreatePubChannelReq)
	if !ok {
		return nil
	}
	return request.BasicData
}

func (adapter *CreatePubChannelProtocolAdapter) GetResponseBasicData(
	responseProtoMsg protoreflect.ProtoMessage) *pb.BasicData {
	response, ok := responseProtoMsg.(*pb.CreatePubChannelRes)
	if !ok {
		return nil
	}
	return response.BasicData
}

func (adapter *CreatePubChannelProtocolAdapter) GetResponseRetCode(
	responseProtoMsg protoreflect.ProtoMessage) *pb.RetCode {
	response, ok := responseProtoMsg.(*pb.CreatePubChannelRes)
	if !ok {
		return nil
	}
	return response.RetCode
}

func (adapter *CreatePubChannelProtocolAdapter) SetRequestSig(
	requestProtoMsg protoreflect.ProtoMessage,
	sig []byte) error {
	request, ok := requestProtoMsg.(*pb.CreatePubChannelReq)
	if !ok {
		return fmt.Errorf("CreatePubChannelProtocolAdapter->SetRequestSig: failed to cast request to *pb.CreatePubChannelReq")
	}
	request.BasicData.Sig = sig
	return nil
}

func (adapter *CreatePubChannelProtocolAdapter) SetResponseSig(
	responseProtoMsg protoreflect.ProtoMessage,
	sig []byte) error {
	response, ok := responseProtoMsg.(*pb.CreatePubChannelRes)
	if !ok {
		return fmt.Errorf("CreatePubChannelProtocolAdapter->SetResponseSig: failed to cast request to *pb.CreatePubChannelRes")
	}
	response.BasicData.Sig = sig
	return nil
}

func (adapter *CreatePubChannelProtocolAdapter) SetResponseRetCode(
	responseProtoMsg protoreflect.ProtoMessage,
	code int32,
	result string) {
	request, ok := responseProtoMsg.(*pb.CreatePubChannelRes)
	if !ok {
		return
	}
	request.RetCode = dmsgProtocol.NewRetCode(code, result)
}

func (adapter *CreatePubChannelProtocolAdapter) CallRequestCallback(
	requestProtoData protoreflect.ProtoMessage) (interface{}, error) {
	data, err := adapter.protocol.Callback.OnCreatePubChannelRequest(requestProtoData)
	return data, err
}

func (adapter *CreatePubChannelProtocolAdapter) CallResponseCallback(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (interface{}, error) {
	data, err := adapter.protocol.Callback.OnCreatePubChannelResponse(requestProtoData, responseProtoData)
	return data, err
}

func NewCreatePubChannelProtocol(
	ctx context.Context,
	host host.Host, protocolCallback common.StreamProtocolCallback, protocolService common.ProtocolService) *common.StreamProtocol {
	adapter := NewCreatePubChannelProtocolAdapter()
	protocol := common.NewStreamProtocol(ctx, host, protocolCallback, protocolService, adapter)
	adapter.protocol = protocol
	return protocol
}
