package protocol

import (
	"context"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/client/common"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
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
	adapter.protocol.Host.SetStreamHandler(dmsgProtocol.PidReleaseMailboxRes, adapter.protocol.OnResponse)
	adapter.protocol.ProtocolRequest = &pb.ReleaseMailboxReq{}
	adapter.protocol.ProtocolResponse = &pb.ReleaseMailboxRes{}
}

func (adapter *ReleaseMailboxProtocolAdapter) GetRequestProtocolID() pb.ProtocolID {
	return pb.ProtocolID_RELEASE_MAILBOX_REQ
}

func (adapter *ReleaseMailboxProtocolAdapter) GetStreamRequestProtocolID() protocol.ID {
	return dmsgProtocol.PidReleaseMailboxReq
}

func (adapter *ReleaseMailboxProtocolAdapter) InitProtocolRequest(basicData *pb.BasicData, dataList ...any) error {
	request := &pb.ReleaseMailboxReq{
		BasicData: basicData,
	}
	adapter.protocol.ProtocolRequest = request
	return nil
}

func (adapter *ReleaseMailboxProtocolAdapter) CallProtocolResponseCallback() (interface{}, error) {
	data, err := adapter.protocol.Callback.OnReleaseMailboxResponse(adapter.protocol.ProtocolResponse)
	return data, err
}

func (adapter *ReleaseMailboxProtocolAdapter) GetProtocolResponseBasicData() *pb.BasicData {
	response, ok := adapter.protocol.ProtocolResponse.(*pb.ReleaseMailboxRes)
	if !ok {
		return nil
	}
	return response.BasicData
}

func (adapter *ReleaseMailboxProtocolAdapter) GetProtocolResponseRetCode() *pb.RetCode {
	response, ok := adapter.protocol.ProtocolResponse.(*pb.ReleaseMailboxRes)
	if !ok {
		return nil
	}
	return response.RetCode
}

func (adapter *ReleaseMailboxProtocolAdapter) SetProtocolRequestSign(signature []byte) {
	request, ok := adapter.protocol.ProtocolRequest.(*pb.ReleaseMailboxReq)
	if !ok {
		return
	}
	request.BasicData.Sign = signature
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
