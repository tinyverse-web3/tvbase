package adapter

import (
	"context"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type SeekMailboxProtocolAdapter struct {
	CommonProtocolAdapter
	protocol *dmsgProtocol.MailboxPProtocol
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
	requestProtoData protoreflect.ProtoMessage,
	basicData *pb.BasicData,
	dataList ...any) (protoreflect.ProtoMessage, error) {
	retCode, err := getRetCode(dataList)
	if err != nil {
		return nil, err
	}
	response := &pb.SeekMailboxRes{
		BasicData: basicData,
		RetCode:   retCode,
	}
	return response, nil
}

func (adapter *SeekMailboxProtocolAdapter) CallRequestCallback(
	requestProtoData protoreflect.ProtoMessage) (any, any, bool, error) {
	data, retCode, abort, err := adapter.protocol.Callback.OnSeekMailboxRequest(requestProtoData)
	return data, retCode, abort, err
}

func (adapter *SeekMailboxProtocolAdapter) CallResponseCallback(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	data, err := adapter.protocol.Callback.OnSeekMailboxResponse(requestProtoData, responseProtoData)
	return data, err
}

func NewSeekMailboxProtocol(
	ctx context.Context,
	host host.Host,
	callback dmsgProtocol.MailboxPpCallback,
	service dmsgProtocol.DmsgService) *dmsgProtocol.MailboxPProtocol {
	adapter := NewSeekMailboxProtocolAdapter()
	protocol := dmsgProtocol.NewMailboxPProtocol(ctx, host, callback, service, adapter)
	adapter.protocol = protocol
	return protocol
}
