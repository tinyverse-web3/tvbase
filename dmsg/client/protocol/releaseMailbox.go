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

type ReleaseMailboxProtocolAdapter struct {
	common.CommonProtocolAdapter
	protocol *common.StreamProtocol
}

func NewReleaseMailboxProtocolAdapter() *ReleaseMailboxProtocolAdapter {
	ret := &ReleaseMailboxProtocolAdapter{}
	return ret
}

func (adapter *ReleaseMailboxProtocolAdapter) init() {
	adapter.protocol.RequestProtoMsg = &pb.ReleaseMailboxReq{}
	adapter.protocol.ResponseProtoMsg = &pb.ReleaseMailboxRes{}
}

func (adapter *ReleaseMailboxProtocolAdapter) GetRequestPID() pb.PID {
	return pb.PID_RELEASE_MAILBOX_REQ
}

func (adapter *ReleaseMailboxProtocolAdapter) GetStreamRequestPID() protocol.ID {
	return dmsgProtocol.PidReleaseMailboxReq
}

func (adapter *ReleaseMailboxProtocolAdapter) GetStreamResponsePID() protocol.ID {
	return dmsgProtocol.PidReleaseMailboxRes
}

func (adapter *ReleaseMailboxProtocolAdapter) InitRequest(basicData *pb.BasicData, dataList ...any) error {
	request := &pb.ReleaseMailboxReq{
		BasicData: basicData,
	}
	adapter.protocol.RequestProtoMsg = request
	return nil
}

func (adapter *ReleaseMailboxProtocolAdapter) CallResponseCallback(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (interface{}, error) {
	data, err := adapter.protocol.Callback.OnReleaseMailboxResponse(requestProtoData, responseProtoData)
	return data, err
}

func (adapter *ReleaseMailboxProtocolAdapter) GetResponseBasicData() *pb.BasicData {
	response, ok := adapter.protocol.ResponseProtoMsg.(*pb.ReleaseMailboxRes)
	if !ok {
		return nil
	}
	return response.BasicData
}

func (adapter *ReleaseMailboxProtocolAdapter) GetResponseRetCode() *pb.RetCode {
	response, ok := adapter.protocol.ResponseProtoMsg.(*pb.ReleaseMailboxRes)
	if !ok {
		return nil
	}
	return response.RetCode
}

func (adapter *ReleaseMailboxProtocolAdapter) SetRequestSig(sig []byte) error {
	request, ok := adapter.protocol.RequestProtoMsg.(*pb.ReleaseMailboxReq)
	if !ok {
		return fmt.Errorf("ReadMailboxMsgProtocolAdapter->SetRequestSig: failed to cast request to *pb.ReleaseMailboxReq")
	}
	request.BasicData.Sig = sig
	return nil
}

func NewReleaseMailboxProtocol(
	ctx context.Context,
	host host.Host,
	protocolCallback common.StreamProtocolCallback,
	protocolService common.ProtocolService) *common.StreamProtocol {
	adapter := NewReleaseMailboxProtocolAdapter()
	protocol := common.NewStreamProtocol(ctx, host, protocolCallback, protocolService, adapter)
	adapter.protocol = protocol
	adapter.init()
	return protocol
}
