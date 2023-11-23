package adapter

import (
	"context"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/basic"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/common"

	"google.golang.org/protobuf/reflect/protoreflect"
)

type ReleaseMailboxProtocolAdapter struct {
	AbstructProtocolAdapter
	protocol *basic.MailboxSProtocol
}

func NewReleaseMailboxProtocolAdapter() *ReleaseMailboxProtocolAdapter {
	ret := &ReleaseMailboxProtocolAdapter{}
	return ret
}

func (adapter *ReleaseMailboxProtocolAdapter) GetRequestPID() pb.PID {
	return pb.PID_RELEASE_MAILBOX_REQ
}

func (adapter *ReleaseMailboxProtocolAdapter) GetResponsePID() pb.PID {
	return pb.PID_RELEASE_MAILBOX_RES
}

func (adapter *ReleaseMailboxProtocolAdapter) GetStreamRequestPID() protocol.ID {
	return common.PidReleaseMailboxReq
}

func (adapter *ReleaseMailboxProtocolAdapter) GetStreamResponsePID() protocol.ID {
	return common.PidReleaseMailboxRes
}

func (adapter *ReleaseMailboxProtocolAdapter) GetEmptyRequest() protoreflect.ProtoMessage {
	return &pb.ReleaseMailboxReq{}
}
func (adapter *ReleaseMailboxProtocolAdapter) GetEmptyResponse() protoreflect.ProtoMessage {
	return &pb.ReleaseMailboxRes{}
}

func (adapter *ReleaseMailboxProtocolAdapter) InitRequest(
	basicData *pb.BasicData,
	dataList ...any) (protoreflect.ProtoMessage, error) {
	requestProtoMsg := &pb.ReleaseMailboxReq{
		BasicData: basicData,
	}
	return requestProtoMsg, nil
}

func (adapter *ReleaseMailboxProtocolAdapter) InitResponse(
	requestProtoData protoreflect.ProtoMessage,
	basicData *pb.BasicData,
	dataList ...any) (protoreflect.ProtoMessage, error) {
	retCode, err := getRetCode(dataList)
	if err != nil {
		return nil, err
	}
	response := &pb.ReleaseMailboxRes{
		BasicData: basicData,
		RetCode:   retCode,
	}
	return response, nil
}

func (adapter *ReleaseMailboxProtocolAdapter) CallRequestCallback(
	requestProtoData protoreflect.ProtoMessage) (any, any, bool, error) {
	data, retCode, abort, err := adapter.protocol.Callback.OnReleaseMailboxRequest(requestProtoData)
	return data, retCode, abort, err
}

func (adapter *ReleaseMailboxProtocolAdapter) CallResponseCallback(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	data, err := adapter.protocol.Callback.OnReleaseMailboxResponse(requestProtoData, responseProtoData)
	return data, err
}

func NewReleaseMailboxProtocol(
	ctx context.Context,
	host host.Host,
	callback common.MailboxSpCallback,
	service common.DmsgService,
	enableRequest bool,
	pubkey string,
) *basic.MailboxSProtocol {
	adapter := NewReleaseMailboxProtocolAdapter()
	protocol := basic.NewMailboxSProtocol(ctx, host, callback, service, adapter, enableRequest, pubkey)
	adapter.protocol = protocol
	return protocol
}
