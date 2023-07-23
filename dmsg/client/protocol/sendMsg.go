package protocol

import (
	"context"
	"errors"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/tinyverse-web3/tvbase/dmsg/client/common"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type SendMsgProtocolAdapter struct {
	common.CommonProtocolAdapter
	protocol *common.PubsubProtocol
}

func NewSendMsgProtocolAdapter() *SendMsgProtocolAdapter {
	ret := &SendMsgProtocolAdapter{}
	return ret
}

func (adapter *SendMsgProtocolAdapter) init() {
	adapter.protocol.RequestProtoMsg = &pb.SendMsgReq{}
	adapter.protocol.ResponseProtoMsg = &pb.SendMsgRes{}
}

func (adapter *SendMsgProtocolAdapter) GetRequestPID() pb.PID {
	return pb.PID_SEND_MSG_REQ
}

func (adapter *SendMsgProtocolAdapter) GetResponsePID() pb.PID {
	return pb.PID_SEND_MSG_RES
}

func (adapter *SendMsgProtocolAdapter) InitRequest(basicData *pb.BasicData, dataList ...any) error {
	if len(dataList) == 1 {
		content, ok := dataList[0].([]byte)
		if !ok {
			return errors.New("SendMsgProtocolAdapter->InitRequest: failed to cast datalist[0] to []byte for content")
		}

		adapter.protocol.RequestProtoMsg = &pb.SendMsgReq{
			BasicData: basicData,
			Content:   content,
		}
	} else {
		return errors.New("SendMsgProtocolAdapter->InitRequest: parameter dataList need contain content")
	}
	return nil
}

func (adapter *SendMsgProtocolAdapter) InitResponse(basicData *pb.BasicData, dataList ...any) error {
	response := &pb.SendMsgRes{
		BasicData: basicData,
		RetCode:   dmsgProtocol.NewSuccRetCode(),
	}
	adapter.protocol.ResponseProtoMsg = response
	return nil
}

func (adapter *SendMsgProtocolAdapter) SetResponseSig(sig []byte) error {
	response, ok := adapter.protocol.ResponseProtoMsg.(*pb.SendMsgRes)
	if !ok {
		return errors.New("SendMsgProtocolAdapter->SetResponseSig: failed to cast request to *pb.SendMsgRes")
	}
	response.BasicData.Sig = sig
	return nil
}

func (adapter *SendMsgProtocolAdapter) CallRequestCallback() (interface{}, error) {
	data, err := adapter.protocol.Callback.OnSendMsgRequest(adapter.protocol.RequestProtoMsg, adapter.protocol.ResponseProtoMsg)
	return data, err
}

func (adapter *SendMsgProtocolAdapter) CallResponseCallback(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (interface{}, error) {
	data, err := adapter.protocol.Callback.OnSendMsgResponse(requestProtoData, responseProtoData)
	return data, err
}

func (adapter *SendMsgProtocolAdapter) GetRequestBasicData() *pb.BasicData {
	request, ok := adapter.protocol.RequestProtoMsg.(*pb.SendMsgReq)
	if !ok {
		return nil
	}
	return request.BasicData
}

func (adapter *SendMsgProtocolAdapter) GetResponseBasicData() *pb.BasicData {
	response, ok := adapter.protocol.ResponseProtoMsg.(*pb.SendMsgRes)
	if !ok {
		return nil
	}
	return response.BasicData
}

func (adapter *SendMsgProtocolAdapter) GetResponseRetCode() *pb.RetCode {
	response, ok := adapter.protocol.ResponseProtoMsg.(*pb.SendMsgRes)
	if !ok {
		return nil
	}
	return response.RetCode
}
func (adapter *SendMsgProtocolAdapter) SetRequestSig(sig []byte) error {
	request, ok := adapter.protocol.RequestProtoMsg.(*pb.SendMsgReq)
	if !ok {
		return errors.New("failed to cast request to *pb.SendMsgReq")
	}
	request.BasicData.Sig = sig
	return nil
}

func NewSendMsgProtocol(
	ctx context.Context,
	host host.Host,
	protocolCallback common.PubsubProtocolCallback,
	dmsgService common.ProtocolService) *common.PubsubProtocol {
	adapter := NewSendMsgProtocolAdapter()
	protocol := common.NewPubsubProtocol(ctx, host, protocolCallback, dmsgService, adapter)
	adapter.protocol = protocol
	adapter.init()
	return protocol
}
