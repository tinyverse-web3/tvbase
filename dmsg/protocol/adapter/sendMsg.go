package adapter

import (
	"context"
	"errors"
	"fmt"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type SendMsgProtocolAdapter struct {
	CommonProtocolAdapter
	Protocol *dmsgProtocol.PubsubProtocol
}

func NewSendMsgProtocolAdapter() *SendMsgProtocolAdapter {
	ret := &SendMsgProtocolAdapter{}
	return ret
}

func (adapter *SendMsgProtocolAdapter) GetRequestPID() pb.PID {
	return pb.PID_SEND_MSG_REQ
}

func (adapter *SendMsgProtocolAdapter) GetResponsePID() pb.PID {
	return pb.PID_SEND_MSG_RES
}

func (adapter *SendMsgProtocolAdapter) GetEmptyRequest() protoreflect.ProtoMessage {
	return &pb.SendMsgReq{}
}
func (adapter *SendMsgProtocolAdapter) GetEmptyResponse() protoreflect.ProtoMessage {
	return &pb.SendMsgRes{}
}

func (adapter *SendMsgProtocolAdapter) InitRequest(
	basicData *pb.BasicData,
	dataList ...any) (protoreflect.ProtoMessage, error) {
	requestProtoMsg := &pb.SendMsgReq{
		BasicData: basicData,
	}
	if len(dataList) == 2 {
		destPubkey, ok := dataList[0].(string)
		if !ok {
			return nil, errors.New("SendMsgProtocolAdapter->InitRequest: failed to cast datalist[0] to []byte for content")
		}

		content, ok := dataList[1].([]byte)
		if !ok {
			return nil, errors.New("SendMsgProtocolAdapter->InitRequest: failed to cast datalist[0] to []byte for content")
		}
		requestProtoMsg.Content = content
		requestProtoMsg.DestPubkey = destPubkey
	} else {
		return requestProtoMsg, errors.New("SendMsgProtocolAdapter->InitRequest: parameter dataList need contain content")
	}
	return requestProtoMsg, nil
}

func (adapter *SendMsgProtocolAdapter) InitResponse(
	requestProtoData protoreflect.ProtoMessage,
	basicData *pb.BasicData,
	dataList ...any) (protoreflect.ProtoMessage, error) {
	var retCode *pb.RetCode
	if len(dataList) > 1 {
		var ok bool
		retCode, ok = dataList[1].(*pb.RetCode)
		if !ok {
			retCode = dmsgProtocol.NewSuccRetCode()
		}
	}
	response := &pb.SendMsgRes{
		BasicData: basicData,
		RetCode:   retCode,
	}
	return response, nil
}

func (adapter *SendMsgProtocolAdapter) GetRequestBasicData(
	requestProtoMsg protoreflect.ProtoMessage) *pb.BasicData {
	request, ok := requestProtoMsg.(*pb.SendMsgReq)
	if !ok {
		return nil
	}
	return request.BasicData
}

func (adapter *SendMsgProtocolAdapter) GetResponseBasicData(
	responseProtoMsg protoreflect.ProtoMessage) *pb.BasicData {
	response, ok := responseProtoMsg.(*pb.SendMsgRes)
	if !ok {
		return nil
	}
	return response.BasicData
}

func (adapter *SendMsgProtocolAdapter) GetResponseRetCode(
	responseProtoMsg protoreflect.ProtoMessage) *pb.RetCode {
	response, ok := responseProtoMsg.(*pb.SendMsgRes)
	if !ok {
		return nil
	}
	return response.RetCode
}

func (adapter *SendMsgProtocolAdapter) SetResponseRetCode(
	responseProtoMsg protoreflect.ProtoMessage,
	code int32,
	result string) {
	request, ok := responseProtoMsg.(*pb.SendMsgRes)
	if !ok {
		return
	}
	request.RetCode = dmsgProtocol.NewRetCode(code, result)
}

func (adapter *SendMsgProtocolAdapter) SetRequestSig(
	requestProtoMsg protoreflect.ProtoMessage,
	sig []byte) error {
	request, ok := requestProtoMsg.(*pb.SendMsgReq)
	if !ok {
		return fmt.Errorf("SendMsgProtocolAdapter->SetRequestSig: failed to cast request to *pb.SendMsgReq")
	}
	request.BasicData.Sig = sig
	return nil
}

func (adapter *SendMsgProtocolAdapter) SetResponseSig(
	responseProtoMsg protoreflect.ProtoMessage,
	sig []byte) error {
	response, ok := responseProtoMsg.(*pb.SendMsgRes)
	if !ok {
		return errors.New("SendMsgProtocolAdapter->SetResponseSig: failed to cast request to *pb.SendMsgRes")
	}
	response.BasicData.Sig = sig
	return nil
}

func (adapter *SendMsgProtocolAdapter) CallRequestCallback(
	requestProtoData protoreflect.ProtoMessage) (any, any, error) {
	data, retCode, err := adapter.Protocol.Callback.OnSendMsgRequest(requestProtoData)
	return data, retCode, err
}

func (adapter *SendMsgProtocolAdapter) CallResponseCallback(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	data, err := adapter.Protocol.Callback.OnSendMsgResponse(requestProtoData, responseProtoData)
	return data, err
}

func NewSendMsgProtocol(
	ctx context.Context,
	host host.Host,
	callback dmsgProtocol.PubsubProtocolCallback,
	service dmsgProtocol.ProtocolService) *dmsgProtocol.PubsubProtocol {
	adapter := NewSendMsgProtocolAdapter()
	protocol := dmsgProtocol.NewPubsubProtocol(ctx, host, callback, service, adapter)
	adapter.Protocol = protocol
	return protocol
}
