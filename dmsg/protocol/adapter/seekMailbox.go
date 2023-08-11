package adapter

import (
	"context"
	"errors"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type SeekMailboxProtocolAdapter struct {
	CommonProtocolAdapter
	protocol *dmsgProtocol.PubsubProtocol
}

func NewSeekMailboxProtocolAdapter() *SeekMailboxProtocolAdapter {
	ret := &SeekMailboxProtocolAdapter{}
	return ret
}

func (adapter *SeekMailboxProtocolAdapter) GetRequestPID() pb.PID {
	return pb.PID_SEEK_MAILBOX_REQ
}

func (adapter *SeekMailboxProtocolAdapter) GetResponsePID() pb.PID {
	return pb.PID_SEEK_MAILBOX_RES
}

func (adapter *SeekMailboxProtocolAdapter) GetEmptyRequest() protoreflect.ProtoMessage {
	return &pb.SeekMailboxReq{}
}
func (adapter *SeekMailboxProtocolAdapter) GetEmptyResponse() protoreflect.ProtoMessage {
	return &pb.SeekMailboxRes{}
}

func (adapter *SeekMailboxProtocolAdapter) InitRequest(
	basicData *pb.BasicData,
	dataList ...any) (protoreflect.ProtoMessage, error) {
	requestProtoMsg := &pb.SeekMailboxReq{
		BasicData: basicData,
	}
	return requestProtoMsg, nil
}

func (adapter *SeekMailboxProtocolAdapter) InitResponse(
	basicData *pb.BasicData,
	dataList ...any) (protoreflect.ProtoMessage, error) {
	responseProtoMsg := &pb.SeekMailboxRes{
		BasicData: basicData,
		RetCode:   dmsgProtocol.NewSuccRetCode(),
	}
	return responseProtoMsg, nil
}

func (adapter *SeekMailboxProtocolAdapter) GetRequestBasicData(
	requestProtoMsg protoreflect.ProtoMessage) *pb.BasicData {
	request, ok := requestProtoMsg.(*pb.SeekMailboxReq)
	if !ok {
		return nil
	}
	return request.BasicData
}

func (adapter *SeekMailboxProtocolAdapter) GetResponseBasicData(
	responseProtoMsg protoreflect.ProtoMessage) *pb.BasicData {
	response, ok := responseProtoMsg.(*pb.SeekMailboxRes)
	if !ok {
		return nil
	}
	return response.BasicData
}

func (adapter *SeekMailboxProtocolAdapter) GetResponseRetCode(
	responseProtoMsg protoreflect.ProtoMessage) *pb.RetCode {
	response, ok := responseProtoMsg.(*pb.SeekMailboxRes)
	if !ok {
		return nil
	}
	return response.RetCode
}

func (adapter *SeekMailboxProtocolAdapter) SetResponseRetCode(
	responseProtoMsg protoreflect.ProtoMessage,
	code int32,
	result string) {
	request, ok := responseProtoMsg.(*pb.SeekMailboxRes)
	if !ok {
		return
	}
	request.RetCode = dmsgProtocol.NewRetCode(code, result)
}

func (adapter *SeekMailboxProtocolAdapter) SetRequestSig(
	requestProtoMsg protoreflect.ProtoMessage,
	sig []byte) error {
	request, ok := requestProtoMsg.(*pb.SeekMailboxReq)
	if !ok {
		return errors.New("SeekMailboxProtocolAdapter->SetRequestSig: failed to cast request to *pb.SeekMailboxReq")
	}
	request.BasicData.Sig = sig
	return nil
}

func (adapter *SeekMailboxProtocolAdapter) SetResponseSig(
	responseProtoMsg protoreflect.ProtoMessage,
	sig []byte) error {
	response, ok := responseProtoMsg.(*pb.SeekMailboxRes)
	if !ok {
		return errors.New("SeekMailboxProtocolAdapter->SetResponseSig: failed to cast request to *pb.SeekMailboxRes")
	}
	response.BasicData.Sig = sig
	return nil
}

func (adapter *SeekMailboxProtocolAdapter) CallRequestCallback(
	requestProtoData protoreflect.ProtoMessage) (interface{}, error) {
	data, err := adapter.protocol.Callback.OnSeekMailboxRequest(requestProtoData)
	return data, err
}

func (adapter *SeekMailboxProtocolAdapter) CallResponseCallback(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (interface{}, error) {
	data, err := adapter.protocol.Callback.OnSeekMailboxResponse(requestProtoData, responseProtoData)
	return data, err
}

func NewSeekMailboxProtocol(
	ctx context.Context,
	host host.Host,
	callback dmsgProtocol.PubsubProtocolCallback,
	service dmsgProtocol.ProtocolService) *dmsgProtocol.PubsubProtocol {
	adapter := NewSeekMailboxProtocolAdapter()
	protocol := dmsgProtocol.NewPubsubProtocol(ctx, host, callback, service, adapter)
	adapter.protocol = protocol
	return protocol
}
