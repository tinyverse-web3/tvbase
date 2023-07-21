package protocol

import (
	"context"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/client/common"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
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
	adapter.protocol.ProtocolRequest = &pb.ReadMailboxReq{}
	adapter.protocol.ProtocolResponse = &pb.ReadMailboxRes{}
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
	adapter.protocol.ProtocolRequest = request
	return nil
}

func (adapter *ReadMailboxMsgProtocolAdapter) CallResponseCallback() (interface{}, error) {
	data, err := adapter.protocol.Callback.OnReadMailboxMsgResponse(adapter.protocol.ProtocolResponse)
	return data, err
}

func (adapter *ReadMailboxMsgProtocolAdapter) GetResponseBasicData() *pb.BasicData {
	response, ok := adapter.protocol.ProtocolResponse.(*pb.ReadMailboxRes)
	if !ok {
		return nil
	}
	return response.BasicData
}

func (adapter *ReadMailboxMsgProtocolAdapter) GetResponseRetCode() *pb.RetCode {
	response, ok := adapter.protocol.ProtocolResponse.(*pb.ReadMailboxRes)
	if !ok {
		return nil
	}
	return response.RetCode
}

func (adapter *ReadMailboxMsgProtocolAdapter) SetRequestSig(sig []byte) {
	request, ok := adapter.protocol.ProtocolRequest.(*pb.ReadMailboxReq)
	if !ok {
		return
	}
	request.BasicData.Sig = sig
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
