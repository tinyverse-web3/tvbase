package protocol

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/client/common"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type CreateMailboxProtocolAdapter struct {
	common.CommonProtocolAdapter
	protocol *common.StreamProtocol
}

func NewCreateMailboxProtocolAdapter() *CreateMailboxProtocolAdapter {
	ret := &CreateMailboxProtocolAdapter{}
	return ret
}

func (adapter *CreateMailboxProtocolAdapter) init() {
	adapter.protocol.RequestProtoMsg = &pb.CreateMailboxReq{}
	adapter.protocol.ResponseProtoMsg = &pb.CreateMailboxRes{}
}

func (adapter *CreateMailboxProtocolAdapter) GetRequestPID() pb.PID {
	return pb.PID_CREATE_MAILBOX_REQ
}

func (adapter *CreateMailboxProtocolAdapter) GetStreamRequestPID() protocol.ID {
	return dmsgProtocol.PidCreateMailboxReq
}

func (adapter *CreateMailboxProtocolAdapter) GetStreamResponsePID() protocol.ID {
	return dmsgProtocol.PidCreateMailboxRes
}

func (adapter *CreateMailboxProtocolAdapter) InitRequest(basicData *pb.BasicData, dataList ...any) error {
	request := &pb.CreateMailboxReq{
		BasicData: basicData,
	}
	adapter.protocol.RequestProtoMsg = request
	return nil
}

func (adapter *CreateMailboxProtocolAdapter) CallResponseCallback(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (interface{}, error) {
	data, err := adapter.protocol.Callback.OnCreateMailboxResponse(requestProtoData, responseProtoData)
	return data, err
}

func (adapter *CreateMailboxProtocolAdapter) GetResponseBasicData() *pb.BasicData {
	response, ok := adapter.protocol.ResponseProtoMsg.(*pb.CreateMailboxRes)
	if !ok {
		return nil
	}
	return response.BasicData
}

func (adapter *CreateMailboxProtocolAdapter) GetResponseRetCode() *pb.RetCode {
	response, ok := adapter.protocol.ResponseProtoMsg.(*pb.CreateMailboxRes)
	if !ok {
		return nil
	}
	return response.RetCode
}

func (adapter *CreateMailboxProtocolAdapter) SetRequestSig(sig []byte) error {
	request, ok := adapter.protocol.RequestProtoMsg.(*pb.CreateMailboxReq)
	if !ok {
		return fmt.Errorf("CreateMailboxProtocolAdapter->SetRequestSig: failed to cast request to *pb.CreateMailboxReq")
	}
	request.BasicData.Sig = sig
	return nil
}

func NewCreateMailboxProtocol(
	ctx context.Context,
	host host.Host, protocolCallback common.StreamProtocolCallback, protocolService common.ProtocolService) *common.StreamProtocol {
	adapter := NewCreateMailboxProtocolAdapter()
	protocol := common.NewStreamProtocol(ctx, host, protocolCallback, protocolService, adapter)
	adapter.protocol = protocol
	adapter.init()
	return protocol
}
