package protocol

import (
	"errors"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/service/common"
)

type SeekMailboxProtocolAdapter struct {
	common.CommonProtocolAdapter
	protocol *common.PubsubProtocol
}

func NewSeekMailboxProtocolAdapter() *SeekMailboxProtocolAdapter {
	ret := &SeekMailboxProtocolAdapter{}
	return ret
}

func (adapter *SeekMailboxProtocolAdapter) init() {
	adapter.protocol.ProtocolRequest = &pb.SeekMailboxReq{}
	adapter.protocol.ProtocolResponse = &pb.SeekMailboxRes{}
}

func (adapter *SeekMailboxProtocolAdapter) GetRequestPID() pb.PID {
	return pb.PID_SEEK_MAILBOX_REQ
}

func (adapter *SeekMailboxProtocolAdapter) GetResponsePID() pb.PID {
	return pb.PID_SEEK_MAILBOX_RES
}

func (adapter *SeekMailboxProtocolAdapter) InitResponse(basicData *pb.BasicData, data interface{}) error {
	response := &pb.SeekMailboxRes{
		BasicData: basicData,
		RetCode:   protocol.NewSuccRetCode(),
	}

	adapter.protocol.ProtocolResponse = response
	return nil
}

func (adapter *SeekMailboxProtocolAdapter) CallRequestCallback() (interface{}, error) {
	data, err := adapter.protocol.Callback.OnSeekMailboxRequest(adapter.protocol.ProtocolRequest)
	return data, err
}

func (adapter *SeekMailboxProtocolAdapter) GetRequestBasicData() *pb.BasicData {
	request, ok := adapter.protocol.ProtocolRequest.(*pb.SeekMailboxReq)
	if !ok {
		return nil
	}
	return request.BasicData
}

func (adapter *SeekMailboxProtocolAdapter) GetResponseBasicData() *pb.BasicData {
	response, ok := adapter.protocol.ProtocolResponse.(*pb.SeekMailboxRes)
	if !ok {
		return nil
	}
	return response.BasicData
}

func (adapter *SeekMailboxProtocolAdapter) GetResponseRetCode() *pb.RetCode {
	response, ok := adapter.protocol.ProtocolResponse.(*pb.SeekMailboxRes)
	if !ok {
		return nil
	}
	return response.RetCode
}

func (adapter *SeekMailboxProtocolAdapter) SetResponseSig(sig []byte) error {
	response, ok := adapter.protocol.ProtocolResponse.(*pb.SeekMailboxRes)
	if !ok {
		return errors.New("failed to cast request to *pb.SeekMailboxRes")
	}
	response.BasicData.Sig = sig
	return nil
}

func NewSeekMailboxProtocol(
	host host.Host,
	protocolCallback common.PubsubProtocolCallback,
	dmsgService common.ProtocolService) *common.PubsubProtocol {
	adapter := NewSeekMailboxProtocolAdapter()
	protocol := common.NewPubsubProtocol(host, dmsgService, protocolCallback, adapter)
	adapter.protocol = protocol
	adapter.init()
	return protocol
}
