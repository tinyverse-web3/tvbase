package protocol

import (
	"context"
	"errors"
	"fmt"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/tinyverse-web3/tvbase/dmsg/client/common"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
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

func (adapter *CustomPubsubProtocolAdapter) GetRequestPID() pb.PID {
	return pb.PID_CUSTOM_PUBSUB_REQ
}

func (adapter *CustomPubsubProtocolAdapter) GetResponsePID() pb.PID {
	return pb.PID_CUSTOM_PUBSUB_RES
}

func (adapter *CustomPubsubProtocolAdapter) GetEmptyRequest() protoreflect.ProtoMessage {
	return &pb.CustomProtocolReq{}
}
func (adapter *CustomPubsubProtocolAdapter) GetEmptyResponse() protoreflect.ProtoMessage {
	return &pb.CustomProtocolRes{}
}

func (adapter *CustomPubsubProtocolAdapter) InitRequest(
	basicData *pb.BasicData,
	dataList ...any) (protoreflect.ProtoMessage, error) {
	var requestProtoMsg *pb.CustomProtocolReq
	if len(dataList) == 2 {
		pid, ok := dataList[0].(string)
		if !ok {
			return nil, errors.New("CustomPubsubProtocolAdapter->InitRequest: failed to cast datalist[0] to string for get customProtocolID")
		}
		content, ok := dataList[1].([]byte)
		if !ok {
			return nil, errors.New("CustomPubsubProtocolAdapter->InitRequest: failed to cast datalist[1] to []byte for get content")
		}
		requestProtoMsg = &pb.CustomProtocolReq{
			BasicData: basicData,
			PID:       pid,
			Content:   content,
		}
	} else {
		return nil, errors.New("CustomPubsubProtocolAdapter->InitRequest: parameter dataList need contain customProtocolID and content")
	}
	return requestProtoMsg, nil
}

func (adapter *CustomPubsubProtocolAdapter) InitResponse(
	basicData *pb.BasicData,
	dataList ...any) (protoreflect.ProtoMessage, error) {
	var responseProtoMsg *pb.CustomProtocolRes
	if len(dataList) == 2 {
		pid, ok := dataList[0].(string)
		if !ok {
			return nil, errors.New("CustomPubsubProtocolAdapter->InitResponse: failed to cast datalist[0] to string for get customProtocolID")
		}
		content, ok := dataList[1].([]byte)
		if !ok {
			return nil, errors.New("CustomPubsubProtocolAdapter->InitResponse: failed to cast datalist[1] to []byte for get content")
		}
		responseProtoMsg = &pb.CustomProtocolRes{
			BasicData: basicData,
			PID:       pid,
			Content:   content,
			RetCode:   dmsgProtocol.NewSuccRetCode(),
		}
	} else {
		return nil, errors.New("CustomPubsubProtocolAdapter->InitResponse: parameter dataList need contain customProtocolID and content")
	}
	return responseProtoMsg, nil
}

func (adapter *CustomPubsubProtocolAdapter) GetRequestBasicData(
	requestProtoMsg protoreflect.ProtoMessage) *pb.BasicData {
	request, ok := requestProtoMsg.(*pb.CustomProtocolReq)
	if !ok {
		return nil
	}
	return request.BasicData
}

func (adapter *CustomPubsubProtocolAdapter) GetResponseBasicData(
	responseProtoMsg protoreflect.ProtoMessage) *pb.BasicData {
	response, ok := responseProtoMsg.(*pb.CustomProtocolRes)
	if !ok {
		return nil
	}
	return response.BasicData
}

func (adapter *CustomPubsubProtocolAdapter) GetResponseRetCode(
	responseProtoMsg protoreflect.ProtoMessage) *pb.RetCode {
	response, ok := responseProtoMsg.(*pb.CustomProtocolRes)
	if !ok {
		return nil
	}
	return response.RetCode
}

func (adapter *CustomPubsubProtocolAdapter) SetRequestSig(
	requestProtoMsg protoreflect.ProtoMessage,
	sig []byte) error {
	request, ok := requestProtoMsg.(*pb.CustomProtocolReq)
	if !ok {
		return fmt.Errorf("CustomPubsubProtocolAdapter->SetRequestSig: failed to cast request to *pb.CustomProtocolReq")
	}
	request.BasicData.Sig = sig
	return nil
}

func (adapter *CustomPubsubProtocolAdapter) SetResponseSig(
	responseProtoMsg protoreflect.ProtoMessage,
	sig []byte) error {
	response, ok := responseProtoMsg.(*pb.CustomProtocolRes)
	if !ok {
		return fmt.Errorf("CreateMailboxProtocolAdapter->SetResponseSig: failed to cast request to *pb.CreateMailboxReq")
	}
	response.BasicData.Sig = sig
	return nil
}

func (adapter *CustomPubsubProtocolAdapter) CallRequestCallback(
	requestProtoData protoreflect.ProtoMessage) (interface{}, error) {
	data, err := adapter.protocol.Callback.OnCustomPubsubProtocolRequest(requestProtoData)
	return data, err
}

func (adapter *CustomPubsubProtocolAdapter) CallResponseCallback(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (interface{}, error) {
	data, err := adapter.protocol.Callback.OnCustomPubsubProtocolResponse(requestProtoData, responseProtoData)
	return data, err
}

func NewCustomPubsubProtocol(
	ctx context.Context,
	host host.Host,
	protocolCallback common.PubsubProtocolCallback,
	dmsgService common.ProtocolService) *common.PubsubProtocol {
	adapter := NewCustomPubsubProtocolAdapter()
	protocol := common.NewPubsubProtocol(ctx, host, protocolCallback, dmsgService, adapter)
	adapter.protocol = protocol
	return protocol
}
