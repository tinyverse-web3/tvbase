package protocol

import (
	"context"
	"errors"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/tinyverse-web3/tvbase/dmsg/client/common"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol"
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

	protocolID := adapter.GetResponseProtocolID()
	adapter.protocol.ClientService.RegPubsubProtocolResCallback(protocolID, adapter.protocol)
}

func (adapter *SeekMailboxProtocolAdapter) GetRequestProtocolID() pb.ProtocolID {
	return pb.ProtocolID_SEEK_MAILBOX_REQ
}

func (adapter *SeekMailboxProtocolAdapter) GetResponseProtocolID() pb.ProtocolID {
	return pb.ProtocolID_SEEK_MAILBOX_RES
}

func (adapter *SeekMailboxProtocolAdapter) GetPubsubSource() common.PubsubSourceType {
	return common.PubsubSource.SrcUser
}

func (adapter *SeekMailboxProtocolAdapter) InitProtocolRequest(basicData *pb.BasicData) {
	request := &pb.SeekMailboxReq{
		BasicData: basicData,
	}
	adapter.protocol.ProtocolRequest = request
}

func (adapter *SeekMailboxProtocolAdapter) CallProtocolResponseCallback() (interface{}, error) {
	data, err := adapter.protocol.Callback.OnSeekMailboxResponse(adapter.protocol.ProtocolResponse)
	return data, err
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
func (adapter *SeekMailboxProtocolAdapter) SetProtocolRequestSign() error {
	signature, err := protocol.SignProtocolMsg(adapter.protocol.ProtocolRequest, adapter.protocol.Host)
	if err != nil {
		return err
	}
	request, ok := adapter.protocol.ProtocolRequest.(*pb.SeekMailboxReq)
	if !ok {
		return errors.New("failed to cast request to *pb.SeekMailboxReq")
	}
	request.BasicData.Sign = signature
	return nil
}

func NewSeekMailboxProtocol(ctx context.Context, host host.Host, protocolCallback common.PubsubProtocolCallback, dmsgService common.ClientService) *common.PubsubProtocol {
	adapter := NewSeekMailboxProtocolAdapter()
	protocol := common.NewPubsubProtocol(ctx, host, protocolCallback, dmsgService, adapter)
	adapter.protocol = protocol
	adapter.init()
	return protocol
}