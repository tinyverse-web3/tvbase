package adapter

import (
	"context"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type CreateMailboxProtocolAdapter struct {
	AbstructProtocolAdapter
	protocol *dmsgProtocol.MailboxSProtocol
}

func NewCreateMailboxProtocolAdapter() *CreateMailboxProtocolAdapter {
	ret := &CreateMailboxProtocolAdapter{}
	return ret
}

func (adapter *CreateMailboxProtocolAdapter) GetRequestPID() pb.PID {
	return pb.PID_CREATE_MAILBOX_REQ
}

func (adapter *CreateMailboxProtocolAdapter) GetResponsePID() pb.PID {
	return pb.PID_CREATE_MAILBOX_RES
}

func (adapter *CreateMailboxProtocolAdapter) GetStreamRequestPID() protocol.ID {
	return dmsgProtocol.PidCreateMailboxReq
}

func (adapter *CreateMailboxProtocolAdapter) GetStreamResponsePID() protocol.ID {
	return dmsgProtocol.PidCreateMailboxRes
}

func (adapter *CreateMailboxProtocolAdapter) GetEmptyRequest() protoreflect.ProtoMessage {
	return &pb.CreateMailboxReq{}
}
func (adapter *CreateMailboxProtocolAdapter) GetEmptyResponse() protoreflect.ProtoMessage {
	return &pb.CreateMailboxRes{}
}

func (adapter *CreateMailboxProtocolAdapter) InitRequest(
	basicData *pb.BasicData,
	dataList ...any) (protoreflect.ProtoMessage, error) {
	request := &pb.CreateMailboxReq{
		BasicData: basicData,
	}
	return request, nil
}

func (adapter *CreateMailboxProtocolAdapter) InitResponse(
	requestProtoData protoreflect.ProtoMessage,
	basicData *pb.BasicData,
	dataList ...any) (protoreflect.ProtoMessage, error) {
	retCode, err := getRetCode(dataList)
	if err != nil {
		return nil, err
	}
	response := &pb.CreateMailboxRes{
		BasicData: basicData,
		RetCode:   retCode,
	}
	return response, nil
}

func (adapter *CreateMailboxProtocolAdapter) CallRequestCallback(
	requestProtoData protoreflect.ProtoMessage) (any, any, bool, error) {
	data, retCode, abort, err := adapter.protocol.Callback.OnCreateMailboxRequest(requestProtoData)
	return data, retCode, abort, err
}

func (adapter *CreateMailboxProtocolAdapter) CallResponseCallback(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	data, err := adapter.protocol.Callback.OnCreateMailboxResponse(requestProtoData, responseProtoData)
	return data, err
}

func NewCreateMailboxProtocol(
	ctx context.Context,
	host host.Host,
	callback dmsgProtocol.MailboxSpCallback,
	service dmsgProtocol.DmsgService,
	enableRequest bool,
	pubkey string,
) *dmsgProtocol.MailboxSProtocol {
	adapter := NewCreateMailboxProtocolAdapter()
	protocol := dmsgProtocol.NewMailboxSProtocol(ctx, host, callback, service, adapter, enableRequest, pubkey)
	adapter.protocol = protocol
	return protocol
}
