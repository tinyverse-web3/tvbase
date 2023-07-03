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

	protocolID := adapter.GetRequestProtocolID()
	adapter.protocol.ProtocolService.RegPubsubProtocolReqCallback(protocolID, adapter.protocol)
}

func (adapter *SeekMailboxProtocolAdapter) GetRequestProtocolID() pb.ProtocolID {
	return pb.ProtocolID_SEEK_MAILBOX_REQ
}

func (adapter *SeekMailboxProtocolAdapter) GetResponseProtocolID() pb.ProtocolID {
	return pb.ProtocolID_SEEK_MAILBOX_RES
}

func (adapter *SeekMailboxProtocolAdapter) InitProtocolResponse(basicData *pb.BasicData) {
	response := &pb.SeekMailboxRes{
		BasicData: basicData,
		RetCode:   protocol.NewSuccRetCode(),
		PeerId:    adapter.protocol.Host.ID().String(),
	}

	adapter.protocol.ProtocolResponse = response
}

func (adapter *SeekMailboxProtocolAdapter) CallProtocolRequestCallback() (interface{}, error) {
	data, err := adapter.protocol.Callback.OnSeekMailboxRequest(adapter.protocol.ProtocolRequest)
	return data, err
}

func (adapter *SeekMailboxProtocolAdapter) GetProtocolRequestBasicData() *pb.BasicData {
	request, ok := adapter.protocol.ProtocolRequest.(*pb.SeekMailboxReq)
	if !ok {
		return nil
	}
	return request.BasicData
}

func (adapter *SeekMailboxProtocolAdapter) GetProtocolResponseBasicData() *pb.BasicData {
	response, ok := adapter.protocol.ProtocolResponse.(*pb.SeekMailboxRes)
	if !ok {
		return nil
	}
	return response.BasicData
}

func (adapter *SeekMailboxProtocolAdapter) GetProtocolResponseRetCode() *pb.RetCode {
	response, ok := adapter.protocol.ProtocolResponse.(*pb.SeekMailboxRes)
	if !ok {
		return nil
	}
	return response.RetCode
}

func (adapter *SeekMailboxProtocolAdapter) SetProtocolResponseSign() error {
	response, ok := adapter.protocol.ProtocolResponse.(*pb.SeekMailboxRes)
	if !ok {
		return errors.New("failed to cast request to *pb.SeekMailboxRes")
	}
	signature, err := protocol.SignProtocolMsg(adapter.protocol.ProtocolResponse, adapter.protocol.Host)
	if err != nil {
		return err
	}
	response.BasicData.Sign = signature
	return nil
}

func NewSeekMailboxProtocol(host host.Host, protocolCallback common.PubsubProtocolCallback, dmsgService common.ProtocolService) *common.PubsubProtocol {
	adapter := NewSeekMailboxProtocolAdapter()
	protocol := common.NewPubsubProtocol(host, protocolCallback, dmsgService, adapter)
	adapter.protocol = protocol
	adapter.init()
	return protocol
}