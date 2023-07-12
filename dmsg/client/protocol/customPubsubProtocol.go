package protocol

import (
	"context"
	"errors"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/tinyverse-web3/tvbase/dmsg/client/common"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
)

type CustomPubsubProtocolAdapter struct {
	common.CommonProtocolAdapter
	protocol *common.PubsubProtocol
}

func NewCustomPubsubProtocolAdapter() *CustomPubsubProtocolAdapter {
	ret := &CustomPubsubProtocolAdapter{}
	return ret
}

func (adapter *CustomPubsubProtocolAdapter) init() {
	adapter.protocol.ProtocolRequest = &pb.CustomProtocolReq{}
	adapter.protocol.ProtocolResponse = &pb.CustomProtocolRes{}

	protocolID := adapter.GetResponseProtocolID()
	adapter.protocol.ProtocolService.RegPubsubProtocolResCallback(protocolID, adapter.protocol)
}

func (adapter *CustomPubsubProtocolAdapter) GetRequestProtocolID() pb.ProtocolID {
	return pb.ProtocolID_SEEK_MAILBOX_REQ
}

func (adapter *CustomPubsubProtocolAdapter) GetResponseProtocolID() pb.ProtocolID {
	return pb.ProtocolID_SEEK_MAILBOX_RES
}

func (adapter *CustomPubsubProtocolAdapter) GetPubsubSource() common.PubsubSourceType {
	return common.PubsubSource.SrcUser
}

func (adapter *CustomPubsubProtocolAdapter) InitProtocolRequest(basicData *pb.BasicData) {
	request := &pb.CustomProtocolReq{
		BasicData: basicData,
	}
	adapter.protocol.ProtocolRequest = request
}

func (adapter *CustomPubsubProtocolAdapter) CallProtocolResponseCallback() (interface{}, error) {
	data, err := adapter.protocol.Callback.OnCustomPubsubProtocolResponse(adapter.protocol.ProtocolRequest, adapter.protocol.ProtocolResponse)
	return data, err
}

func (adapter *CustomPubsubProtocolAdapter) GetProtocolResponseBasicData() *pb.BasicData {
	response, ok := adapter.protocol.ProtocolResponse.(*pb.CustomProtocolRes)
	if !ok {
		return nil
	}
	return response.BasicData
}

func (adapter *CustomPubsubProtocolAdapter) GetProtocolResponseRetCode() *pb.RetCode {
	response, ok := adapter.protocol.ProtocolResponse.(*pb.CustomProtocolRes)
	if !ok {
		return nil
	}
	return response.RetCode
}
func (adapter *CustomPubsubProtocolAdapter) SetProtocolRequestSign(signature []byte) error {
	request, ok := adapter.protocol.ProtocolRequest.(*pb.CustomProtocolReq)
	if !ok {
		return errors.New("failed to cast request to *pb.CustomProtocolReq")
	}
	request.BasicData.Sign = signature
	return nil
}

func NewCustomPubsubProtocol(
	ctx context.Context,
	host host.Host,
	protocolCallback common.PubsubProtocolCallback,
	dmsgService common.ProtocolService) *common.PubsubProtocol {
	adapter := NewCustomPubsubProtocolAdapter()
	protocol := common.NewPubsubProtocol(ctx, host, protocolCallback, dmsgService, adapter)
	adapter.protocol = protocol
	adapter.init()
	return protocol
}
