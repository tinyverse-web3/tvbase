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

type ReleaseMailboxProtocolAdapter struct {
	CommonProtocolAdapter
	protocol *dmsgProtocol.MailboxSProtocol
}

func NewReleaseMailboxProtocolAdapter() *ReleaseMailboxProtocolAdapter {
	ret := &ReleaseMailboxProtocolAdapter{}
	return ret
}

func (adapter *ReleaseMailboxProtocolAdapter) GetRequestPID() pb.PID {
	return pb.PID_RELEASE_MAILBOX_REQ
}

func (adapter *ReleaseMailboxProtocolAdapter) GetResponsePID() pb.PID {
	return pb.PID_RELEASE_MAILBOX_RES
}

func (adapter *ReleaseMailboxProtocolAdapter) GetStreamRequestPID() protocol.ID {
	return dmsgProtocol.PidReleaseMailboxReq
}

func (adapter *ReleaseMailboxProtocolAdapter) GetStreamResponsePID() protocol.ID {
	return dmsgProtocol.PidReleaseMailboxRes
}

func (adapter *ReleaseMailboxProtocolAdapter) GetEmptyRequest() protoreflect.ProtoMessage {
	return &pb.ReleaseMailboxReq{}
}
func (adapter *ReleaseMailboxProtocolAdapter) GetEmptyResponse() protoreflect.ProtoMessage {
	return &pb.ReleaseMailboxRes{}
}

func (adapter *ReleaseMailboxProtocolAdapter) InitRequest(
	basicData *pb.BasicData,
	dataList ...any) (protoreflect.ProtoMessage, error) {
	requestProtoMsg := &pb.ReleaseMailboxReq{
		BasicData: basicData,
	}
	return requestProtoMsg, nil
}

func (adapter *ReleaseMailboxProtocolAdapter) InitResponse(
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
	response := &pb.ReleaseMailboxRes{
		BasicData: basicData,
		RetCode:   retCode,
	}
	return response, nil
}

func (adapter *ReleaseMailboxProtocolAdapter) GetRequestBasicData(
	requestProtoMsg protoreflect.ProtoMessage) *pb.BasicData {
	request, ok := requestProtoMsg.(*pb.ReleaseMailboxReq)
	if !ok {
		return nil
	}
	return request.BasicData
}

func (adapter *ReleaseMailboxProtocolAdapter) GetResponseBasicData(
	responseProtoMsg protoreflect.ProtoMessage) *pb.BasicData {
	response, ok := responseProtoMsg.(*pb.ReleaseMailboxRes)
	if !ok {
		return nil
	}
	return response.BasicData
}

func (adapter *ReleaseMailboxProtocolAdapter) GetResponseRetCode(
	responseProtoMsg protoreflect.ProtoMessage) *pb.RetCode {
	response, ok := responseProtoMsg.(*pb.ReleaseMailboxRes)
	if !ok {
		return nil
	}
	return response.RetCode
}

func (adapter *ReleaseMailboxProtocolAdapter) SetResponseRetCode(
	responseProtoMsg protoreflect.ProtoMessage,
	code int32,
	result string) {
	request, ok := responseProtoMsg.(*pb.ReleaseMailboxRes)
	if !ok {
		return
	}
	request.RetCode = dmsgProtocol.NewRetCode(code, result)
}

func (adapter *ReleaseMailboxProtocolAdapter) SetRequestSig(
	requestProtoMsg protoreflect.ProtoMessage,
	sig []byte) error {
	request, ok := requestProtoMsg.(*pb.ReleaseMailboxReq)
	if !ok {
		return fmt.Errorf("ReleaseMailboxProtocolAdapter->SetRequestSig: failed to cast request to *pb.ReleaseMailboxReq")
	}
	request.BasicData.Sig = sig
	return nil
}

func (adapter *ReleaseMailboxProtocolAdapter) SetResponseSig(
	responseProtoMsg protoreflect.ProtoMessage, sig []byte) error {
	response, ok := responseProtoMsg.(*pb.ReleaseMailboxRes)
	if !ok {
		return fmt.Errorf("ReleaseMailboxProtocolAdapter->SetResponseSig: failed to cast request to *pb.ReleaseMailboxRes")
	}
	response.BasicData.Sig = sig
	return nil
}

func (adapter *ReleaseMailboxProtocolAdapter) CallRequestCallback(
	requestProtoData protoreflect.ProtoMessage) (any, any, error) {
	data, retCode, err := adapter.protocol.Callback.OnReleaseMailboxRequest(requestProtoData)
	return data, retCode, err
}

func (adapter *ReleaseMailboxProtocolAdapter) CallResponseCallback(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	data, err := adapter.protocol.Callback.OnReleaseMailboxResponse(requestProtoData, responseProtoData)
	return data, err
}

func NewReleaseMailboxProtocol(
	ctx context.Context,
	host host.Host,
	callback dmsgProtocol.MailboxSpCallback,
	service dmsgProtocol.DmsgServiceInterface) *dmsgProtocol.MailboxSProtocol {
	adapter := NewReleaseMailboxProtocolAdapter()
	protocol := dmsgProtocol.NewMailboxSProtocol(ctx, host, callback, service, adapter)
	adapter.protocol = protocol
	return protocol
}
