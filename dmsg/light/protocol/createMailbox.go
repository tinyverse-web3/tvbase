package protocol

import (
	"context"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/light/common"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
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
	adapter.protocol.Host.SetStreamHandler(dmsgProtocol.PidCreateMailboxRes, adapter.protocol.OnResponse)
	adapter.protocol.ProtocolRequest = &pb.CreateMailboxReq{}
	adapter.protocol.ProtocolResponse = &pb.CreateMailboxRes{}
}

func (adapter *CreateMailboxProtocolAdapter) GetRequestProtocolID() pb.ProtocolID {
	return pb.ProtocolID_CREATE_MAILBOX_REQ
}

func (adapter *CreateMailboxProtocolAdapter) GetStreamRequestProtocolID() protocol.ID {
	return dmsgProtocol.PidCreateMailboxReq
}

func (adapter *CreateMailboxProtocolAdapter) InitProtocolRequest(basicData *pb.BasicData) {
	request := &pb.CreateMailboxReq{
		BasicData: basicData,
	}
	adapter.protocol.ProtocolRequest = request
}

func (adapter *CreateMailboxProtocolAdapter) CallProtocolResponseCallback() (interface{}, error) {
	data, err := adapter.protocol.Callback.OnCreateMailboxResponse(adapter.protocol.ProtocolResponse)
	return data, err
}

func (adapter *CreateMailboxProtocolAdapter) GetProtocolResponseBasicData() *pb.BasicData {
	response, ok := adapter.protocol.ProtocolResponse.(*pb.CreateMailboxRes)
	if !ok {
		return nil
	}
	return response.BasicData
}

func (adapter *CreateMailboxProtocolAdapter) GetProtocolResponseRetCode() *pb.RetCode {
	response, ok := adapter.protocol.ProtocolResponse.(*pb.CreateMailboxRes)
	if !ok {
		return nil
	}
	return response.RetCode
}

func (adapter *CreateMailboxProtocolAdapter) SetProtocolRequestSign(signature []byte) {
	request, ok := adapter.protocol.ProtocolRequest.(*pb.CreateMailboxReq)
	if !ok {
		return
	}
	request.BasicData.Sign = signature
}

func NewCreateMailboxProtocol(ctx context.Context, host host.Host, protocolCallback common.StreamProtocolCallback) *common.StreamProtocol {
	adapter := NewCreateMailboxProtocolAdapter()
	protocol := common.NewStreamProtocol(ctx, host, protocolCallback, adapter)
	adapter.protocol = protocol
	adapter.init()
	return protocol
}
