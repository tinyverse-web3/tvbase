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

type PubChannelProtocolAdapter struct {
	common.CommonProtocolAdapter
	protocol *common.StreamProtocol
}

func NewCreatePubChannelProtocolAdapter() *PubChannelProtocolAdapter {
	ret := &PubChannelProtocolAdapter{}
	return ret
}

func (adapter *PubChannelProtocolAdapter) init() {
	adapter.protocol.Request = &pb.CreatePubChannelReq{}
}

func (adapter *PubChannelProtocolAdapter) GetResponsePID() pb.PID {
	return pb.PID_CREATE_PUB_CHANNEL_RES
}

func (adapter *PubChannelProtocolAdapter) GetStreamResponsePID() protocol.ID {
	return dmsgProtocol.PidCreatePubChannelRes
}

func (adapter *PubChannelProtocolAdapter) GetStreamRequestPID() protocol.ID {
	return dmsgProtocol.PidCreatePubChannelReq
}

func (adapter *PubChannelProtocolAdapter) DestoryProtocol() {
	adapter.protocol.Host.RemoveStreamHandler(dmsgProtocol.PidCreatePubChannelReq)
}

func (adapter *PubChannelProtocolAdapter) SetProtocolResponseFailRet(errMsg string) {
	request, ok := adapter.protocol.Response.(*pb.CreatePubChannelRes)
	if !ok {
		return
	}
	request.RetCode = dmsgProtocol.NewFailRetCode(errMsg)
}

func (adapter *PubChannelProtocolAdapter) SetProtocolResponseRet(code int32, result string) {
	request, ok := adapter.protocol.Response.(*pb.CreatePubChannelRes)
	if !ok {
		return
	}
	request.RetCode = dmsgProtocol.NewRetCode(code, result)
}

func (adapter *PubChannelProtocolAdapter) GetRequestBasicData() *pb.BasicData {
	request, ok := adapter.protocol.Request.(*pb.CreatePubChannelReq)
	if !ok {
		return nil
	}
	return request.BasicData
}

func (adapter *PubChannelProtocolAdapter) GetResponseBasicData() *pb.BasicData {
	request, ok := adapter.protocol.Response.(*pb.CreatePubChannelRes)
	if !ok {
		return nil
	}
	return request.BasicData
}

func (adapter *PubChannelProtocolAdapter) InitResponse(basicData *pb.BasicData, data interface{}) error {
	response := &pb.CreatePubChannelRes{
		BasicData: basicData,
		RetCode:   dmsgProtocol.NewSuccRetCode(),
	}
	adapter.protocol.Response = response
	return nil
}

func (adapter *PubChannelProtocolAdapter) SetResponseSig(sig []byte) error {
	response, ok := adapter.protocol.Response.(*pb.CreatePubChannelRes)
	if !ok {
		return errors.New("failed to cast request to *pb.CreatePubChannelRes")
	}
	response.BasicData.Sig = sig
	return nil
}

func (adapter *PubChannelProtocolAdapter) CallRequestCallback() (interface{}, error) {
	data, err := adapter.protocol.Callback.OnCreatePubChannelRequest(adapter.protocol.Request)
	return data, err
}

func NewCreatePubChannelProtocol(ctx context.Context, host host.Host, protocolService common.ProtocolService, protocolCallback common.StreamProtocolCallback) *common.StreamProtocol {
	adapter := NewCreatePubChannelProtocolAdapter()
	protocol := common.NewStreamProtocol(ctx, host, protocolService, protocolCallback, adapter)
	adapter.protocol = protocol
	adapter.init()
	return protocol
}
