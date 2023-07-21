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

type CreateMailboxProtocolAdapter struct {
	common.CommonProtocolAdapter
	protocol *common.StreamProtocol
}

func NewCreateMailboxProtocolAdapter() *CreateMailboxProtocolAdapter {
	ret := &CreateMailboxProtocolAdapter{}
	return ret
}

func (adapter *CreateMailboxProtocolAdapter) init() {
	adapter.protocol.ProtocolRequest = &pb.CreateMailboxReq{}
}

func (adapter *CreateMailboxProtocolAdapter) GetResponsePID() pb.PID {
	return pb.PID_CREATE_MAILBOX_RES
}

func (adapter *CreateMailboxProtocolAdapter) GetStreamResponsePID() protocol.ID {
	return dmsgProtocol.PidCreateMailboxRes
}

func (adapter *CreateMailboxProtocolAdapter) GetStreamRequestPID() protocol.ID {
	return dmsgProtocol.PidCreateMailboxReq
}

func (adapter *CreateMailboxProtocolAdapter) DestoryProtocol() {
	adapter.protocol.Host.RemoveStreamHandler(dmsgProtocol.PidCreateMailboxReq)
}

func (adapter *CreateMailboxProtocolAdapter) SetProtocolResponseFailRet(errMsg string) {
	request, ok := adapter.protocol.ProtocolResponse.(*pb.CreateMailboxRes)
	if !ok {
		return
	}
	request.RetCode = dmsgProtocol.NewFailRetCode(errMsg)
}

func (adapter *CreateMailboxProtocolAdapter) SetProtocolResponseRet(code int32, result string) {
	request, ok := adapter.protocol.ProtocolResponse.(*pb.CreateMailboxRes)
	if !ok {
		return
	}
	request.RetCode = dmsgProtocol.NewRetCode(code, result)
}

func (adapter *CreateMailboxProtocolAdapter) GetRequestBasicData() *pb.BasicData {
	request, ok := adapter.protocol.ProtocolRequest.(*pb.CreateMailboxReq)
	if !ok {
		return nil
	}
	return request.BasicData
}

func (adapter *CreateMailboxProtocolAdapter) GetResponseBasicData() *pb.BasicData {
	request, ok := adapter.protocol.ProtocolResponse.(*pb.CreateMailboxRes)
	if !ok {
		return nil
	}
	return request.BasicData
}

func (adapter *CreateMailboxProtocolAdapter) InitResponse(basicData *pb.BasicData, data any) error {
	response := &pb.CreateMailboxRes{
		BasicData: basicData,
		RetCode:   dmsgProtocol.NewSuccRetCode(),
	}
	adapter.protocol.ProtocolResponse = response
	return nil
}

func (adapter *CreateMailboxProtocolAdapter) SetResponseSig(sig []byte) error {
	response, ok := adapter.protocol.ProtocolResponse.(*pb.CreateMailboxRes)
	if !ok {
		return errors.New("failed to cast request to *pb.CreateMailboxRes")
	}
	response.BasicData.Sig = sig
	return nil
}

func (adapter *CreateMailboxProtocolAdapter) CallRequestCallback() (any, error) {
	data, err := adapter.protocol.Callback.OnCreateMailboxRequest(adapter.protocol.ProtocolRequest)
	return data, err
}

func (adapter *CreateMailboxProtocolAdapter) CallResponseCallback() (any, error) {
	data, err := adapter.protocol.Callback.OnCreateMailboxResponse(adapter.protocol.ProtocolResponse)
	return data, err
}

func NewCreateMailboxProtocol(ctx context.Context, host host.Host, protocolService common.ProtocolService, protocolCallback common.StreamProtocolCallback) *common.StreamProtocol {
	adapter := NewCreateMailboxProtocolAdapter()
	protocol := common.NewStreamProtocol(ctx, host, protocolService, protocolCallback, adapter)
	adapter.protocol = protocol
	adapter.init()
	return protocol
}
