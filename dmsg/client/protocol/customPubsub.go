package protocol

import (
	"context"
	"errors"
	"fmt"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/tinyverse-web3/tvbase/dmsg/client/common"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	"google.golang.org/protobuf/reflect/protoreflect"
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
	adapter.protocol.RequestProtoMsg = &pb.CustomProtocolReq{}
	adapter.protocol.ResponseProtoMsg = &pb.CustomProtocolRes{}
}

func (adapter *CustomPubsubProtocolAdapter) GetRequestPID() pb.PID {
	return pb.PID_CUSTOM_PUBSUB_PROTOCOL_REQ
}

func (adapter *CustomPubsubProtocolAdapter) GetResponsePID() pb.PID {
	return pb.PID_CUSTOM_PUBSUB_PROTOCOL_RES
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

		adapter.protocol.RequestProtoMsg = &pb.CustomProtocolReq{
			BasicData: basicData,
			PID:       pid,
			Content:   content,
		}
	} else {
		return errors.New("CustomPubsubProtocolAdapter->InitRequest: parameter dataList need contain customProtocolID and content")
	}
	return nil
}

func (adapter *CustomPubsubProtocolAdapter) CallRequestCallback() (interface{}, error) {
	data, err := adapter.protocol.Callback.OnCustomPubsubProtocolRequest(adapter.protocol.RequestProtoMsg, adapter.protocol.ResponseProtoMsg)
	return data, err
}

func (adapter *CustomPubsubProtocolAdapter) CallResponseCallback(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (interface{}, error) {
	data, err := adapter.protocol.Callback.OnCustomPubsubProtocolResponse(requestProtoData, responseProtoData)
	return data, err
}

func (adapter *CustomPubsubProtocolAdapter) GetResponseBasicData() *pb.BasicData {
	response, ok := adapter.protocol.ResponseProtoMsg.(*pb.CustomProtocolRes)
	if !ok {
		return nil
	}
	return response.BasicData
}

func (adapter *CustomPubsubProtocolAdapter) GetResponseRetCode() *pb.RetCode {
	response, ok := adapter.protocol.ResponseProtoMsg.(*pb.CustomProtocolRes)
	if !ok {
		return nil
	}
	return response.RetCode
}
func (adapter *CustomPubsubProtocolAdapter) SetRequestSig(sig []byte) error {
	request, ok := adapter.protocol.RequestProtoMsg.(*pb.CustomProtocolReq)
	if !ok {
		return fmt.Errorf("CustomPubsubProtocolAdapter->SetRequestSig: failed to cast request to *pb.CustomProtocolReq")
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
