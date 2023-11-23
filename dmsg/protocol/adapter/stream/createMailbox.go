package stream

import (
	"context"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	basicAdapter "github.com/tinyverse-web3/tvbase/dmsg/protocol/adapter/basic"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/adapter/util"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/basic"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/common"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/newProtocol"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type CreateMailboxProtocolAdapter struct {
	basicAdapter.AbstructProtocolAdapter
	protocol *basic.MailboxSProtocol
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
	return common.PidCreateMailboxReq
}

func (adapter *CreateMailboxProtocolAdapter) GetStreamResponsePID() protocol.ID {
	return common.PidCreateMailboxRes
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
	retCode, err := util.GetRetCode(dataList)
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
	callback common.MailboxSpCallback,
	service common.DmsgService,
	enableRequest bool,
	pubkey string,
) *basic.MailboxSProtocol {
	adapter := NewCreateMailboxProtocolAdapter()
	protocol := newProtocol.NewMailboxSProtocol(ctx, host, callback, service, adapter, enableRequest, pubkey)
	adapter.protocol = protocol
	return protocol
}
