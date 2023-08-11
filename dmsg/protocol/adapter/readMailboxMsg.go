package adapter

import (
	"context"
	"errors"
	"fmt"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type ReadMailboxMsgProtocolAdapter struct {
	CommonProtocolAdapter
	protocol *dmsgProtocol.StreamProtocol
}

func NewReadMailboxMsgProtocolAdapter() *ReadMailboxMsgProtocolAdapter {
	ret := &ReadMailboxMsgProtocolAdapter{}
	return ret
}

func (adapter *ReadMailboxMsgProtocolAdapter) GetRequestPID() pb.PID {
	return pb.PID_READ_MAILBOX_REQ
}

func (adapter *ReadMailboxMsgProtocolAdapter) GetResponsePID() pb.PID {
	return pb.PID_READ_MAILBOX_RES
}

func (adapter *ReadMailboxMsgProtocolAdapter) GetStreamRequestPID() protocol.ID {
	return dmsgProtocol.PidReadMailboxMsgReq
}

func (adapter *ReadMailboxMsgProtocolAdapter) GetStreamResponsePID() protocol.ID {
	return dmsgProtocol.PidReadMailboxMsgRes
}

func (adapter *ReadMailboxMsgProtocolAdapter) GetEmptyRequest() protoreflect.ProtoMessage {
	return &pb.ReadMailboxReq{}
}
func (adapter *ReadMailboxMsgProtocolAdapter) GetEmptyResponse() protoreflect.ProtoMessage {
	return &pb.ReadMailboxRes{}
}

func (adapter *ReadMailboxMsgProtocolAdapter) InitRequest(
	basicData *pb.BasicData,
	dataList ...any) (protoreflect.ProtoMessage, error) {
	requestProtoMsg := &pb.ReadMailboxReq{
		BasicData: basicData,
	}
	return requestProtoMsg, nil
}

func (adapter *ReadMailboxMsgProtocolAdapter) InitResponse(
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
	response := &pb.ReadMailboxRes{
		BasicData: basicData,
		RetCode:   retCode,
	}

	if len(dataList) < 1 {
		return nil, errors.New("ReadMailboxMsgProtocolAdapter->InitResponse: dataList need contain MailboxItemList")
	}
	mailboxItemList, ok := dataList[0].([]*pb.MailboxItem)
	if !ok {
		return response, errors.New("ReadMailboxMsgProtocolAdapter->InitResponse: fail to cast dataList[0] to []*pb.MailboxMsgData")
	}

	response.ContentList = mailboxItemList

	return response, nil
}

func (adapter *ReadMailboxMsgProtocolAdapter) GetRequestBasicData(
	requestProtoMsg protoreflect.ProtoMessage) *pb.BasicData {
	request, ok := requestProtoMsg.(*pb.ReadMailboxReq)
	if !ok {
		return nil
	}
	return request.BasicData
}

func (adapter *ReadMailboxMsgProtocolAdapter) GetResponseBasicData(
	responseProtoMsg protoreflect.ProtoMessage) *pb.BasicData {
	response, ok := responseProtoMsg.(*pb.ReadMailboxRes)
	if !ok {
		return nil
	}
	return response.BasicData
}

func (adapter *ReadMailboxMsgProtocolAdapter) GetResponseRetCode(
	responseProtoMsg protoreflect.ProtoMessage) *pb.RetCode {
	response, ok := responseProtoMsg.(*pb.ReadMailboxRes)
	if !ok {
		return nil
	}
	return response.RetCode
}

func (adapter *ReadMailboxMsgProtocolAdapter) SetResponseRetCode(
	responseProtoMsg protoreflect.ProtoMessage,
	code int32,
	result string) {
	request, ok := responseProtoMsg.(*pb.ReadMailboxRes)
	if !ok {
		return
	}
	request.RetCode = dmsgProtocol.NewRetCode(code, result)
}

func (adapter *ReadMailboxMsgProtocolAdapter) SetRequestSig(
	requestProtoMsg protoreflect.ProtoMessage, sig []byte) error {
	request, ok := requestProtoMsg.(*pb.ReadMailboxReq)
	if !ok {
		return fmt.Errorf("ReadMailboxMsgProtocolAdapter->SetRequestSig: failed to cast request to *pb.ReadMailboxReq")
	}
	request.BasicData.Sig = sig
	return nil
}

func (adapter *ReadMailboxMsgProtocolAdapter) SetResponseSig(
	responseProtoMsg protoreflect.ProtoMessage, sig []byte) error {
	response, ok := responseProtoMsg.(*pb.ReadMailboxRes)
	if !ok {
		return fmt.Errorf("ReadMailboxMsgProtocolAdapter->SetResponseSig: failed to cast request to *pb.ReadMailboxRes")
	}
	response.BasicData.Sig = sig
	return nil
}

func (adapter *ReadMailboxMsgProtocolAdapter) CallRequestCallback(
	requestProtoData protoreflect.ProtoMessage) (any, any, error) {
	data, retCode, err := adapter.protocol.Callback.OnReadMailboxMsgRequest(requestProtoData)
	return data, retCode, err
}

func (adapter *ReadMailboxMsgProtocolAdapter) CallResponseCallback(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	data, err := adapter.protocol.Callback.OnReadMailboxMsgResponse(requestProtoData, responseProtoData)
	return data, err
}

func NewReadMailboxMsgProtocol(
	ctx context.Context,
	host host.Host,
	callback dmsgProtocol.StreamProtocolCallback,
	service dmsgProtocol.ProtocolService) *dmsgProtocol.StreamProtocol {
	adapter := NewReadMailboxMsgProtocolAdapter()
	protocol := dmsgProtocol.NewStreamProtocol(ctx, host, callback, service, adapter)
	adapter.protocol = protocol
	return protocol
}
