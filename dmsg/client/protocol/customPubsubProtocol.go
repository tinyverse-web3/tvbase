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
}

func (adapter *CustomPubsubProtocolAdapter) GetRequestPID() pb.PID {
	return pb.PID_CUSTOM_PUBSUB_PROTOCOL_REQ
}

func (adapter *CustomPubsubProtocolAdapter) GetResponsePID() pb.PID {
	return pb.PID_CUSTOM_PUBSUB_PROTOCOL_RES
}

func (adapter *CustomPubsubProtocolAdapter) GetPubsubSource() common.PubsubSourceType {
	return common.PubsubSource.SrcUser
}

func (adapter *CustomPubsubProtocolAdapter) InitRequest(basicData *pb.BasicData, dataList ...any) error {
	if len(dataList) == 2 {
		pid, ok := dataList[0].(string)
		if !ok {
			return errors.New("CustomPubsubProtocolAdapter->InitRequest: failed to cast datalist[0] to string for get customProtocolID")
		}
		content, ok := dataList[1].([]byte)
		if !ok {
			return errors.New("CustomPubsubProtocolAdapter->InitRequest: failed to cast datalist[1] to []byte for get content")
		}

		adapter.protocol.ProtocolRequest = &pb.CustomProtocolReq{
			BasicData: basicData,
			PID:       pid,
			Content:   content,
		}
	} else {
		return errors.New("CustomPubsubProtocolAdapter->InitRequest: parameter dataList need contain customProtocolID and content")
	}
	return nil
}

func (adapter *CustomPubsubProtocolAdapter) CallRequestCallback() (bool, interface{}, error) {
	needResponse, data, err := adapter.protocol.Callback.OnCustomPubsubProtocolRequest(adapter.protocol.ProtocolRequest, adapter.protocol.ProtocolResponse)
	return needResponse, data, err
}

func (adapter *CustomPubsubProtocolAdapter) CallResponseCallback() (interface{}, error) {
	data, err := adapter.protocol.Callback.OnCustomPubsubProtocolResponse(adapter.protocol.ProtocolRequest, adapter.protocol.ProtocolResponse)
	return data, err
}

func (adapter *CustomPubsubProtocolAdapter) GetResponseBasicData() *pb.BasicData {
	response, ok := adapter.protocol.ProtocolResponse.(*pb.CustomProtocolRes)
	if !ok {
		return nil
	}
	return response.BasicData
}

func (adapter *CustomPubsubProtocolAdapter) GetResponseRetCode() *pb.RetCode {
	response, ok := adapter.protocol.ProtocolResponse.(*pb.CustomProtocolRes)
	if !ok {
		return nil
	}
	return response.RetCode
}
func (adapter *CustomPubsubProtocolAdapter) SetRequestSig(sig []byte) error {
	request, ok := adapter.protocol.ProtocolRequest.(*pb.CustomProtocolReq)
	if !ok {
		return errors.New("failed to cast request to *pb.CustomProtocolReq")
	}
	request.BasicData.Sig = sig
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
