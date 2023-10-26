package protocol

import (
	"context"
	"errors"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/service/common"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type CreateMailboxProtocolAdapter struct {
	common.CommonProtocolAdapter
	protocol *common.StreamProtocol
}

func NewCreateMailboxProtocolAdapter() *CreateMailboxProtocolAdapter {
	ret := &CreateMailboxProtocolAdapter{}
	return ret
}

func (adapter *CreateMailboxProtocolAdapter) init() {
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

func (adapter *CreateMailboxProtocolAdapter) GetRequestBasicData(requestProtoData protoreflect.ProtoMessage) *pb.BasicData {
	request, ok := requestProtoData.(*pb.CreateMailboxReq)
	if !ok {
		return nil
	}
	return request.BasicData
}

func (adapter *CreateMailboxProtocolAdapter) GetResponseBasicData(responseProtoData protoreflect.ProtoMessage) *pb.BasicData {
	response, ok := responseProtoData.(*pb.CreateMailboxRes)
	if !ok {
		return nil
	}
	return response.BasicData
}

func (adapter *CreateMailboxProtocolAdapter) InitResponse(
	requestProtoData protoreflect.ProtoMessage,
	basicData *pb.BasicData,
	dataList ...any) (protoreflect.ProtoMessage, error) {
	var retCode *pb.RetCode
	var ok bool
	if len(dataList) > 1 {
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

func (adapter *CreateMailboxProtocolAdapter) SetResponseSig(responseProtoData protoreflect.ProtoMessage, sig []byte) error {
	response, ok := responseProtoData.(*pb.CreateMailboxRes)
	if !ok {
		return errors.New("CreateMailboxProtocolAdapter->SetResponseSig: failed to cast response to *pb.CreateMailboxRes")
	}
	response.BasicData.Sig = sig
	return nil
}

func (adapter *CreateMailboxProtocolAdapter) CallRequestCallback(requestProtoData protoreflect.ProtoMessage) (any, any, error) {
	data, retCode, err := adapter.protocol.Callback.OnCreateMailboxRequest(requestProtoData)
	return data, retCode, err
}

func (adapter *CreateMailboxProtocolAdapter) CallResponseCallback(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	data, err := adapter.protocol.Callback.OnCreateMailboxResponse(responseProtoData)
	return data, err
}

func NewCreateMailboxProtocol(ctx context.Context, host host.Host, protocolService common.ProtocolService, protocolCallback common.StreamProtocolCallback) *common.StreamProtocol {
	adapter := NewCreateMailboxProtocolAdapter()
	protocol := common.NewStreamProtocol(ctx, host, protocolService, protocolCallback, adapter)
	adapter.protocol = protocol
	adapter.init()
	return protocol
}
