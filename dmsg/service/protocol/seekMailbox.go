package protocol

import (
	"errors"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/service/common"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type SeekMailboxProtocolAdapter struct {
	common.CommonProtocolAdapter
	protocol *common.PubsubProtocol
}

func NewSeekMailboxProtocolAdapter() *SeekMailboxProtocolAdapter {
	ret := &SeekMailboxProtocolAdapter{}
	return ret
}

func (adapter *SeekMailboxProtocolAdapter) init() {
}

func (adapter *SeekMailboxProtocolAdapter) GetRequestPID() pb.PID {
	return pb.PID_SEEK_MAILBOX_REQ
}

func (adapter *SeekMailboxProtocolAdapter) GetResponsePID() pb.PID {
	return pb.PID_SEEK_MAILBOX_RES
}

func (adapter *SeekMailboxProtocolAdapter) GetEmptyRequest() protoreflect.ProtoMessage {
	return &pb.SeekMailboxReq{}
}
func (adapter *SeekMailboxProtocolAdapter) GetEmptyResponse() protoreflect.ProtoMessage {
	return &pb.SeekMailboxRes{}
}

func (adapter *SeekMailboxProtocolAdapter) GetRequestBasicData(requestProtoData protoreflect.ProtoMessage) *pb.BasicData {
	request, ok := requestProtoData.(*pb.SeekMailboxReq)
	if !ok {
		return nil
	}
	return request.BasicData
}

func (adapter *SeekMailboxProtocolAdapter) GetResponseBasicData(responseProtoData protoreflect.ProtoMessage) *pb.BasicData {
	response, ok := responseProtoData.(*pb.SeekMailboxRes)
	if !ok {
		return nil
	}
	return response.BasicData
}

func (adapter *SeekMailboxProtocolAdapter) InitResponse(
	requestProtoData protoreflect.ProtoMessage,
	basicData *pb.BasicData,
	dataList ...any) (protoreflect.ProtoMessage, error) {
	response := &pb.SeekMailboxRes{
		BasicData: basicData,
		RetCode:   protocol.NewSuccRetCode(),
	}

	return response, nil
}

func (adapter *SeekMailboxProtocolAdapter) SetResponseSig(responseProtoData protoreflect.ProtoMessage, sig []byte) error {
	response, ok := responseProtoData.(*pb.SeekMailboxRes)
	if !ok {
		return errors.New("SeekMailboxProtocolAdapter->SetResponseSig: failed to cast response to *pb.SeekMailboxRes")
	}
	response.BasicData.Sig = sig
	return nil
}

func NewSeekMailboxProtocol(
	host host.Host,
	protocolCallback common.PubsubProtocolCallback,
	dmsgService common.ProtocolService) *common.PubsubProtocol {
	adapter := NewSeekMailboxProtocolAdapter()
	protocol := common.NewPubsubProtocol(host, dmsgService, protocolCallback, adapter)
	adapter.protocol = protocol
	adapter.init()
	return protocol
}
