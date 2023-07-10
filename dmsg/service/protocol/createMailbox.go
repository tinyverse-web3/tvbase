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
	adapter.protocol.Host.SetStreamHandler(dmsgProtocol.PidCreateMailboxReq, adapter.protocol.OnRequest)
	adapter.protocol.ProtocolRequest = &pb.CreateMailboxReq{}
}

func (adapter *CreateMailboxProtocolAdapter) GetResponseProtocolID() pb.ProtocolID {
	return pb.ProtocolID_CREATE_MAILBOX_RES
}

func (adapter *CreateMailboxProtocolAdapter) GetStreamResponseProtocolID() protocol.ID {
	return dmsgProtocol.PidCreateMailboxRes
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

func (adapter *CreateMailboxProtocolAdapter) GetProtocolRequestBasicData() *pb.BasicData {
	request, ok := adapter.protocol.ProtocolRequest.(*pb.CreateMailboxReq)
	if !ok {
		return nil
	}
	return request.BasicData
}

func (adapter *CreateMailboxProtocolAdapter) GetProtocolResponseBasicData() *pb.BasicData {
	request, ok := adapter.protocol.ProtocolResponse.(*pb.CreateMailboxRes)
	if !ok {
		return nil
	}
	return request.BasicData
}

func (adapter *CreateMailboxProtocolAdapter) InitProtocolResponse(basicData *pb.BasicData, data interface{}) error {
	response := &pb.CreateMailboxRes{
		BasicData: basicData,
		RetCode:   dmsgProtocol.NewSuccRetCode(),
	}
	adapter.protocol.ProtocolResponse = response
	return nil
}

func (adapter *CreateMailboxProtocolAdapter) SetProtocolResponseSign(signature []byte) error {
	response, ok := adapter.protocol.ProtocolResponse.(*pb.CreateMailboxRes)
	if !ok {
		return errors.New("failed to cast request to *pb.CreateMailboxRes")
	}
	response.BasicData.Sign = signature
	return nil
}

func (adapter *CreateMailboxProtocolAdapter) CallProtocolRequestCallback() (interface{}, error) {
	data, err := adapter.protocol.Callback.OnCreateMailboxRequest(adapter.protocol.ProtocolRequest)
	return data, err
}

func (adapter *CreateMailboxProtocolAdapter) CallProtocolResponseCallback() (interface{}, error) {
	data, err := adapter.protocol.Callback.OnCreateMailboxResponse(adapter.protocol.ProtocolResponse)
	return data, err
}

func NewCreateMailboxProtocol(ctx context.Context, host host.Host, protocolCallback common.StreamProtocolCallback) *common.StreamProtocol {
	adapter := NewCreateMailboxProtocolAdapter()
	protocol := common.NewStreamProtocol(ctx, host, protocolCallback, adapter)
	adapter.protocol = protocol
	adapter.init()
	return protocol
}
