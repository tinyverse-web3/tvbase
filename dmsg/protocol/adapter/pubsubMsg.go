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

type PubsubMsgProtocolAdapter struct {
	CommonProtocolAdapter
	Protocol *dmsgProtocol.PubsubMsgProtocol
}

func NewPubsubMsgProtocolAdapter() *PubsubMsgProtocolAdapter {
	ret := &PubsubMsgProtocolAdapter{}
	return ret
}

func (adapter *PubsubMsgProtocolAdapter) GetRequestPID() pb.PID {
	return pb.PID_MSG_REQ
}

func (adapter *PubsubMsgProtocolAdapter) GetResponsePID() pb.PID {
	return pb.PID_MSG_RES
}

func (adapter *PubsubMsgProtocolAdapter) GetEmptyRequest() protoreflect.ProtoMessage {
	return &pb.MsgReq{}
}
func (adapter *PubsubMsgProtocolAdapter) GetEmptyResponse() protoreflect.ProtoMessage {
	return &pb.MsgRes{}
}

func (adapter *PubsubMsgProtocolAdapter) InitRequest(
	basicData *pb.BasicData,
	dataList ...any) (protoreflect.ProtoMessage, error) {
	requestProtoMsg := &pb.MsgReq{
		BasicData: basicData,
	}
	if len(dataList) == 2 {
		destPubkey, ok := dataList[0].(string)
		if !ok {
			return nil, errors.New("PubsubMsgProtocolAdapter->InitRequest: failed to cast datalist[0] to []byte for content")
		}

		content, ok := dataList[1].([]byte)
		if !ok {
			return nil, errors.New("PubsubMsgProtocolAdapter->InitRequest: failed to cast datalist[0] to []byte for content")
		}
		requestProtoMsg.Content = content
		requestProtoMsg.DestPubkey = destPubkey
	} else {
		return requestProtoMsg, errors.New("PubsubMsgProtocolAdapter->InitRequest: parameter dataList need contain content")
	}
	return requestProtoMsg, nil
}

func (adapter *PubsubMsgProtocolAdapter) InitResponse(
	requestProtoData protoreflect.ProtoMessage,
	basicData *pb.BasicData,
	dataList ...any) (protoreflect.ProtoMessage, error) {
	var content []byte
	if len(dataList) > 0 && dataList[0] != nil {
		var ok bool
		content, ok = dataList[0].([]byte)
		if !ok {
			return nil, fmt.Errorf("PubsubMsgProtocolAdapter->InitResponse: fail to cast dataList[0] to []byte")
		}
	}
	retCode, err := getRetCode(dataList)
	if err != nil {
		return nil, err
	}
	response := &pb.MsgRes{
		BasicData: basicData,
		Content:   content,
		RetCode:   retCode,
	}
	return response, nil
}

func (adapter *PubsubMsgProtocolAdapter) GetRequestBasicData(
	requestProtoMsg protoreflect.ProtoMessage) *pb.BasicData {
	request, ok := requestProtoMsg.(*pb.MsgReq)
	if !ok {
		return nil
	}
	return request.BasicData
}

func (adapter *PubsubMsgProtocolAdapter) GetResponseBasicData(
	responseProtoMsg protoreflect.ProtoMessage) *pb.BasicData {
	response, ok := responseProtoMsg.(*pb.MsgRes)
	if !ok {
		return nil
	}
	return response.BasicData
}

func (adapter *PubsubMsgProtocolAdapter) GetResponseRetCode(
	responseProtoMsg protoreflect.ProtoMessage) *pb.RetCode {
	response, ok := responseProtoMsg.(*pb.MsgRes)
	if !ok {
		return nil
	}
	return response.RetCode
}

func (adapter *PubsubMsgProtocolAdapter) SetResponseRetCode(
	responseProtoMsg protoreflect.ProtoMessage,
	code int32,
	result string) {
	request, ok := responseProtoMsg.(*pb.MsgRes)
	if !ok {
		return
	}
	request.RetCode = dmsgProtocol.NewRetCode(code, result)
}

func (adapter *PubsubMsgProtocolAdapter) SetRequestSig(
	requestProtoMsg protoreflect.ProtoMessage,
	sig []byte) error {
	request, ok := requestProtoMsg.(*pb.MsgReq)
	if !ok {
		return fmt.Errorf("PubsubMsgProtocolAdapter->SetRequestSig: failed to cast request to *pb.MsgReq")
	}
	request.BasicData.Sig = sig
	return nil
}

func (adapter *PubsubMsgProtocolAdapter) SetResponseSig(
	responseProtoMsg protoreflect.ProtoMessage,
	sig []byte) error {
	response, ok := responseProtoMsg.(*pb.MsgRes)
	if !ok {
		return errors.New("PubsubMsgProtocolAdapter->SetResponseSig: failed to cast request to *pb.MsgRes")
	}
	response.BasicData.Sig = sig
	return nil
}

func (adapter *PubsubMsgProtocolAdapter) CallRequestCallback(
	requestProtoData protoreflect.ProtoMessage) (any, any, bool, error) {
	data, retCode, abort, err := adapter.Protocol.Callback.OnPubsubMsgRequest(requestProtoData)
	return data, retCode, abort, err
}

func (adapter *PubsubMsgProtocolAdapter) CallResponseCallback(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	data, err := adapter.Protocol.Callback.OnPubsubMsgResponse(requestProtoData, responseProtoData)
	return data, err
}

func NewPubsubMsgProtocol(
	ctx context.Context,
	host host.Host,
	callback dmsgProtocol.PubsubMsgCallback,
	service dmsgProtocol.DmsgServiceInterface) *dmsgProtocol.PubsubMsgProtocol {
	adapter := NewPubsubMsgProtocolAdapter()
	protocol := dmsgProtocol.NewPubsubMsgProtocol(ctx, host, callback, service, adapter)
	adapter.Protocol = protocol
	return protocol
}
