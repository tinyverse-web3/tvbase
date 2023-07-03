package protocol

import (
	"context"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/service/common"
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
	adapter.protocol.Host.SetStreamHandler(dmsgProtocol.PidReleaseMailboxReq, adapter.protocol.OnRequest)
	adapter.protocol.ProtocolRequest = &pb.ReleaseMailboxReq{}
}

func (adapter *ReleaseMailboxProtocolAdapter) GetResponseProtocolID() pb.ProtocolID {
	return pb.ProtocolID_RELEASE_MAILBOX_RES
}

func (adapter *ReleaseMailboxProtocolAdapter) GetStreamResponseProtocolID() protocol.ID {
	return dmsgProtocol.PidReleaseMailboxRes
}

func (adapter *ReleaseMailboxProtocolAdapter) DestoryProtocol() {
	adapter.protocol.Host.RemoveStreamHandler(dmsgProtocol.PidReleaseMailboxReq)
}

func (adapter *ReleaseMailboxProtocolAdapter) SetProtocolResponseFailRet(errMsg string) {
	request, ok := adapter.protocol.ProtocolResponse.(*pb.ReleaseMailboxRes)
	if !ok {
		return
	}
	request.RetCode = dmsgProtocol.NewFailRetCode(errMsg)
}

func (adapter *ReleaseMailboxProtocolAdapter) SetProtocolResponseRet(code int32, result string) {
	request, ok := adapter.protocol.ProtocolResponse.(*pb.ReleaseMailboxRes)
	if !ok {
		return
	}
	request.RetCode = dmsgProtocol.NewRetCode(code, result)
}

func (adapter *ReleaseMailboxProtocolAdapter) GetProtocolRequestBasicData() *pb.BasicData {
	request, ok := adapter.protocol.ProtocolRequest.(*pb.ReleaseMailboxReq)
	if !ok {
		return nil
	}
	return request.BasicData
}

func (adapter *ReleaseMailboxProtocolAdapter) GetProtocolResponseBasicData() *pb.BasicData {
	request, ok := adapter.protocol.ProtocolResponse.(*pb.ReleaseMailboxRes)
	if !ok {
		return nil
	}
	return request.BasicData
}

func (adapter *ReleaseMailboxProtocolAdapter) InitProtocolResponse(basicData *pb.BasicData, data interface{}) error {
	response := &pb.ReleaseMailboxRes{
		BasicData: basicData,
		RetCode:   dmsgProtocol.NewSuccRetCode(),
	}
	adapter.protocol.ProtocolResponse = response
	return nil
}

func (adapter *ReleaseMailboxProtocolAdapter) SetProtocolResponseSign(signature []byte) {
	response, ok := adapter.protocol.ProtocolResponse.(*pb.ReleaseMailboxRes)
	if !ok {
		return
	}
	response.BasicData.Sign = signature
}

func (adapter *ReleaseMailboxProtocolAdapter) CallProtocolRequestCallback() (interface{}, error) {
	data, err := adapter.protocol.Callback.OnReleaseMailboxRequest(adapter.protocol.ProtocolRequest)
	return data, err
}

func NewReleaseMailboxProtocol(ctx context.Context, host host.Host, protocolCallback common.StreamProtocolCallback) *common.StreamProtocol {
	adapter := NewReleaseMailboxProtocolAdapter()
	protocol := common.NewStreamProtocol(ctx, host, protocolCallback, adapter)
	adapter.protocol = protocol
	adapter.init()
	return protocol
}
