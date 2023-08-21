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

type ReleaseMailboxProtocolAdapter struct {
	common.CommonProtocolAdapter
	protocol *common.StreamProtocol
}

func NewReleaseMailboxProtocolAdapter() *ReleaseMailboxProtocolAdapter {
	ret := &ReleaseMailboxProtocolAdapter{}
	return ret
}

func (adapter *ReleaseMailboxProtocolAdapter) init() {
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

func (adapter *ReleaseMailboxProtocolAdapter) GetRequestBasicData(requestProtoData protoreflect.ProtoMessage) *pb.BasicData {
	request, ok := requestProtoData.(*pb.ReleaseMailboxReq)
	if !ok {
		return nil
	}
	return request.BasicData
}

func (adapter *ReleaseMailboxProtocolAdapter) GetResponseBasicData(responseProtoData protoreflect.ProtoMessage) *pb.BasicData {
	response, ok := responseProtoData.(*pb.ReleaseMailboxRes)
	if !ok {
		return nil
	}
	return response.BasicData
}

func (adapter *ReleaseMailboxProtocolAdapter) InitResponse(
	requestProtoData protoreflect.ProtoMessage,
	basicData *pb.BasicData,
	dataList ...any) (protoreflect.ProtoMessage, error) {
	response := &pb.ReleaseMailboxRes{
		BasicData: basicData,
		RetCode:   dmsgProtocol.NewSuccRetCode(),
	}

	return response, nil
}

func (adapter *ReleaseMailboxProtocolAdapter) SetResponseSig(responseProtoData protoreflect.ProtoMessage, sig []byte) error {
	response, ok := responseProtoData.(*pb.ReleaseMailboxRes)
	if !ok {
		return errors.New("CreateMailboxProtocolAdapter->SetResponseSig: failed to cast response to *pb.ReleaseMailboxRes")
	}
	response.BasicData.Sig = sig
	return nil
}

func (adapter *ReleaseMailboxProtocolAdapter) CallRequestCallback(requestProtoData protoreflect.ProtoMessage) (any, any, error) {
	data, retCode, err := adapter.protocol.Callback.OnReleaseMailboxRequest(requestProtoData)
	return data, retCode, err
}

func NewReleaseMailboxProtocol(ctx context.Context, host host.Host, protocolService common.ProtocolService, protocolCallback common.StreamProtocolCallback) *common.StreamProtocol {
	adapter := NewReleaseMailboxProtocolAdapter()
	protocol := common.NewStreamProtocol(ctx, host, protocolService, protocolCallback, adapter)
	adapter.protocol = protocol
	adapter.init()
	return protocol
}
