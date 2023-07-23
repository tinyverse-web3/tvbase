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

type ReadMailboxMsgProtocolAdapter struct {
	common.CommonProtocolAdapter
	protocol *common.StreamProtocol
}

func NewReadMailboxMsgProtocolAdapter() *ReadMailboxMsgProtocolAdapter {
	ret := &ReadMailboxMsgProtocolAdapter{}
	return ret
}

func (adapter *ReadMailboxMsgProtocolAdapter) init() {
	adapter.protocol.RequestProtoMsg = &pb.ReadMailboxReq{}
	adapter.protocol.ResponseProtoMsg = &pb.ReadMailboxRes{}
}

func (adapter *ReadMailboxMsgProtocolAdapter) GetRequestPID() pb.PID {
	return pb.PID_READ_MAILBOX_MSG_REQ
}

func (adapter *ReadMailboxMsgProtocolAdapter) GetStreamRequestPID() protocol.ID {
	return dmsgProtocol.PidReadMailboxMsgReq
}

func (adapter *ReadMailboxMsgProtocolAdapter) GetStreamResponsePID() protocol.ID {
	return dmsgProtocol.PidReadMailboxMsgRes
}

func (adapter *ReadMailboxMsgProtocolAdapter) InitRequest(basicData *pb.BasicData, dataList ...any) error {
	request := &pb.ReadMailboxReq{
		BasicData: basicData,
	}
	adapter.protocol.RequestProtoMsg = request
	return nil
}

func (adapter *ReadMailboxMsgProtocolAdapter) CallResponseCallback(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (interface{}, error) {
	data, err := adapter.protocol.Callback.OnReadMailboxMsgResponse(requestProtoData, responseProtoData)
	return data, err
}

func (adapter *ReadMailboxMsgProtocolAdapter) GetResponseBasicData() *pb.BasicData {
	response, ok := adapter.protocol.ResponseProtoMsg.(*pb.ReadMailboxRes)
	if !ok {
		return nil
	}
	return response.BasicData
}

func (adapter *ReadMailboxMsgProtocolAdapter) GetResponseRetCode() *pb.RetCode {
	response, ok := adapter.protocol.ResponseProtoMsg.(*pb.ReadMailboxRes)
	if !ok {
		return nil
	}
	return response.RetCode
}

func (adapter *ReadMailboxMsgProtocolAdapter) SetRequestSig(sig []byte) error {
	request, ok := adapter.protocol.RequestProtoMsg.(*pb.ReadMailboxReq)
	if !ok {
		return fmt.Errorf("ReadMailboxMsgProtocolAdapter->SetRequestSig: failed to cast request to *pb.ReadMailboxReq")
	}
	request.BasicData.Sig = sig
	return nil
}

func NewReadMailboxMsgProtocol(
	ctx context.Context,
	host host.Host,
	protocolCallback common.StreamProtocolCallback,
	protocolService common.ProtocolService) *common.StreamProtocol {
	adapter := NewReadMailboxMsgProtocolAdapter()
	protocol := common.NewStreamProtocol(ctx, host, protocolCallback, protocolService, adapter)
	adapter.protocol = protocol
	adapter.init()
	return protocol
}
