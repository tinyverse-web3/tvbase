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
	adapter.protocol.ProtocolRequest = &pb.ReadMailboxMsgReq{}
	adapter.protocol.ProtocolResponse = &pb.ReadMailboxMsgRes{}
}

func (adapter *ReadMailboxMsgProtocolAdapter) GetRequestProtocolID() pb.ProtocolID {
	return pb.ProtocolID_READ_MAILBOX_MSG_REQ
}

func (adapter *ReadMailboxMsgProtocolAdapter) GetStreamRequestProtocolID() protocol.ID {
	return dmsgProtocol.PidReadMailboxMsgReq
}

func (adapter *ReadMailboxMsgProtocolAdapter) GetStreamResponseProtocolID() protocol.ID {
	return dmsgProtocol.PidReadMailboxMsgRes
}

func (adapter *ReadMailboxMsgProtocolAdapter) InitProtocolRequest(basicData *pb.BasicData, dataList ...any) error {
	request := &pb.ReadMailboxMsgReq{
		BasicData: basicData,
	}
	adapter.protocol.ProtocolRequest = request
	return nil
}

func (adapter *ReadMailboxMsgProtocolAdapter) CallProtocolResponseCallback() (interface{}, error) {
	data, err := adapter.protocol.Callback.OnReadMailboxMsgResponse(adapter.protocol.ProtocolResponse)
	return data, err
}

func (adapter *ReadMailboxMsgProtocolAdapter) GetProtocolResponseBasicData() *pb.BasicData {
	response, ok := adapter.protocol.ProtocolResponse.(*pb.ReadMailboxMsgRes)
	if !ok {
		return nil
	}
	return response.BasicData
}

func (adapter *ReadMailboxMsgProtocolAdapter) GetProtocolResponseRetCode() *pb.RetCode {
	response, ok := adapter.protocol.ProtocolResponse.(*pb.ReadMailboxMsgRes)
	if !ok {
		return nil
	}
	return response.RetCode
}

func (adapter *ReadMailboxMsgProtocolAdapter) SetProtocolRequestSign(signature []byte) {
	request, ok := adapter.protocol.ProtocolRequest.(*pb.ReadMailboxMsgReq)
	if !ok {
		return
	}
	request.BasicData.Sign = signature
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
