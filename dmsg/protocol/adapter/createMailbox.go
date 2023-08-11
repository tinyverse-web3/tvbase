package adapter

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"

	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type CreateMailboxProtocolAdapter struct {
	CommonProtocolAdapter
	protocol *dmsgProtocol.StreamProtocol
}

func NewCreateMailboxProtocolAdapter() *CreateMailboxProtocolAdapter {
	ret := &CreateMailboxProtocolAdapter{}
	return ret
}

func (adapter *CreateMailboxProtocolAdapter) GetRequestPID() pb.PID {
	return pb.PID_CREATE_MAILBOX_REQ
}

func (adapter *CreateMailboxProtocolAdapter) GetResponsePID() pb.PID {
	return pb.PID_CREATE_MAILBOX_RES
}

func (adapter *CreateMailboxProtocolAdapter) GetStreamRequestPID() protocol.ID {
	return dmsgProtocol.PidCreateMailboxReq
}

func (adapter *CreateMailboxProtocolAdapter) GetStreamResponsePID() protocol.ID {
	return dmsgProtocol.PidCreateMailboxRes
}

func (adapter *CreateMailboxProtocolAdapter) GetEmptyRequest() protoreflect.ProtoMessage {
	return &pb.CreateMailboxReq{}
}
func (adapter *CreateMailboxProtocolAdapter) GetEmptyResponse() protoreflect.ProtoMessage {
	return &pb.CreateMailboxRes{}
}

func (adapter *CreateMailboxProtocolAdapter) InitRequest(
	basicData *pb.BasicData,
	dataList ...any) (protoreflect.ProtoMessage, error) {
	request := &pb.CreateMailboxReq{
		BasicData: basicData,
	}
	return request, nil
}

func (adapter *CreateMailboxProtocolAdapter) InitResponse(
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
	response := &pb.CreateMailboxRes{
		BasicData: basicData,
		RetCode:   retCode,
	}
	return response, nil
}

func (adapter *CreateMailboxProtocolAdapter) GetRequestBasicData(
	requestProtoMsg protoreflect.ProtoMessage) *pb.BasicData {
	request, ok := requestProtoMsg.(*pb.CreateMailboxReq)
	if !ok {
		return nil
	}
	return request.BasicData
}

func (adapter *CreateMailboxProtocolAdapter) GetResponseBasicData(
	responseProtoMsg protoreflect.ProtoMessage) *pb.BasicData {
	response, ok := responseProtoMsg.(*pb.CreateMailboxRes)
	if !ok {
		return nil
	}
	return response.BasicData
}

func (adapter *CreateMailboxProtocolAdapter) GetResponseRetCode(
	responseProtoMsg protoreflect.ProtoMessage) *pb.RetCode {
	response, ok := responseProtoMsg.(*pb.CreateMailboxRes)
	if !ok {
		return nil
	}
	return response.RetCode
}

func (adapter *CreateMailboxProtocolAdapter) SetRequestSig(
	requestProtoMsg protoreflect.ProtoMessage,
	sig []byte) error {
	request, ok := requestProtoMsg.(*pb.CreateMailboxReq)
	if !ok {
		return fmt.Errorf("CreateMailboxProtocolAdapter->SetRequestSig: failed to cast request to *pb.CreateMailboxReq")
	}
	request.BasicData.Sig = sig
	return nil
}

func (adapter *CreateMailboxProtocolAdapter) SetResponseSig(
	responseProtoMsg protoreflect.ProtoMessage,
	sig []byte) error {
	response, ok := responseProtoMsg.(*pb.CreateMailboxRes)
	if !ok {
		return fmt.Errorf("CreateMailboxProtocolAdapter->SetResponseSig: failed to cast request to *pb.CreateMailboxRes")
	}
	response.BasicData.Sig = sig
	return nil
}

func (adapter *CreateMailboxProtocolAdapter) SetResponseRetCode(
	responseProtoMsg protoreflect.ProtoMessage,
	code int32,
	result string) {
	request, ok := responseProtoMsg.(*pb.CreateMailboxRes)
	if !ok {
		return
	}
	request.RetCode = dmsgProtocol.NewRetCode(code, result)
}

func (adapter *CreateMailboxProtocolAdapter) CallRequestCallback(
	requestProtoData protoreflect.ProtoMessage) (any, any, error) {
	data, retCode, err := adapter.protocol.Callback.OnCreateMailboxRequest(requestProtoData)
	return data, retCode, err
}

func (adapter *CreateMailboxProtocolAdapter) CallResponseCallback(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	data, err := adapter.protocol.Callback.OnCreateMailboxResponse(requestProtoData, responseProtoData)
	return data, err
}

func NewCreateMailboxProtocol(
	ctx context.Context,
	host host.Host,
	callback dmsgProtocol.StreamProtocolCallback,
	service dmsgProtocol.ProtocolService) *dmsgProtocol.StreamProtocol {
	adapter := NewCreateMailboxProtocolAdapter()
	protocol := dmsgProtocol.NewStreamProtocol(ctx, host, callback, service, adapter)
	adapter.protocol = protocol
	return protocol
}
