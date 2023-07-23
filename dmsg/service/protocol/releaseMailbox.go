package protocol

import (
	"context"
	"errors"

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
	adapter.protocol.Request = &pb.ReleaseMailboxReq{}
}

func (adapter *ReleaseMailboxProtocolAdapter) GetResponsePID() pb.PID {
	return pb.PID_RELEASE_MAILBOX_RES
}

func (adapter *ReleaseMailboxProtocolAdapter) GetStreamResponsePID() protocol.ID {
	return dmsgProtocol.PidReleaseMailboxRes
}

func (adapter *ReleaseMailboxProtocolAdapter) DestoryProtocol() {
	adapter.protocol.Host.RemoveStreamHandler(dmsgProtocol.PidReleaseMailboxReq)
}

func (adapter *ReleaseMailboxProtocolAdapter) SetProtocolResponseFailRet(errMsg string) {
	request, ok := adapter.protocol.Response.(*pb.ReleaseMailboxRes)
	if !ok {
		return
	}
	request.RetCode = dmsgProtocol.NewFailRetCode(errMsg)
}

func (adapter *ReleaseMailboxProtocolAdapter) SetProtocolResponseRet(code int32, result string) {
	request, ok := adapter.protocol.Response.(*pb.ReleaseMailboxRes)
	if !ok {
		return
	}
	request.RetCode = dmsgProtocol.NewRetCode(code, result)
}

func (adapter *ReleaseMailboxProtocolAdapter) GetRequestBasicData() *pb.BasicData {
	request, ok := adapter.protocol.Request.(*pb.ReleaseMailboxReq)
	if !ok {
		return nil
	}
	return request.BasicData
}

func (adapter *ReleaseMailboxProtocolAdapter) GetResponseBasicData() *pb.BasicData {
	request, ok := adapter.protocol.Response.(*pb.ReleaseMailboxRes)
	if !ok {
		return nil
	}
	return request.BasicData
}

func (adapter *ReleaseMailboxProtocolAdapter) InitResponse(basicData *pb.BasicData, data interface{}) error {
	response := &pb.ReleaseMailboxRes{
		BasicData: basicData,
		RetCode:   dmsgProtocol.NewSuccRetCode(),
	}
	adapter.protocol.Response = response
	return nil
}

func (adapter *ReleaseMailboxProtocolAdapter) SetResponseSig(sig []byte) error {
	response, ok := adapter.protocol.Response.(*pb.ReleaseMailboxRes)
	if !ok {
		return errors.New("failed to cast request to *pb.ReleaseMailboxRes")
	}
	response.BasicData.Sig = sig
	return nil
}

func (adapter *ReleaseMailboxProtocolAdapter) CallRequestCallback() (interface{}, error) {
	data, err := adapter.protocol.Callback.OnReleaseMailboxRequest(adapter.protocol.Request)
	return data, err
}

func NewReleaseMailboxProtocol(ctx context.Context, host host.Host, protocolService common.ProtocolService, protocolCallback common.StreamProtocolCallback) *common.StreamProtocol {
	adapter := NewReleaseMailboxProtocolAdapter()
	protocol := common.NewStreamProtocol(ctx, host, protocolService, protocolCallback, adapter)
	adapter.protocol = protocol
	adapter.init()
	return protocol
}
