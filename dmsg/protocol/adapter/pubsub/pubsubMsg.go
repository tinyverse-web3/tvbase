package pubsub

import (
	"context"
	"errors"
	"fmt"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	basicAdapter "github.com/tinyverse-web3/tvbase/dmsg/protocol/adapter/basic"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/adapter/util"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/basic"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/common"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/newProtocol"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type PubsubMsgProtocolAdapter struct {
	basicAdapter.AbstructProtocolAdapter
	Protocol *basic.PubsubMsgProtocol
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
	retCode, err := util.GetRetCode(dataList)
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
	callback common.PubsubMsgCallback,
	service common.DmsgService) *basic.PubsubMsgProtocol {
	adapter := NewPubsubMsgProtocolAdapter()
	protocol := newProtocol.NewPubsubMsgProtocol(ctx, host, callback, service, adapter)
	adapter.Protocol = protocol
	return protocol
}
