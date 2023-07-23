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

type SeekMailboxProtocolAdapter struct {
	common.CommonProtocolAdapter
	protocol *common.PubsubProtocol
}

func NewSeekMailboxProtocolAdapter() *SeekMailboxProtocolAdapter {
	ret := &SeekMailboxProtocolAdapter{}
	return ret
}

func (adapter *SeekMailboxProtocolAdapter) init() {
	adapter.protocol.ProtocolRequest = &pb.SeekMailboxReq{}
	adapter.protocol.ProtocolResponse = &pb.SeekMailboxRes{}
}

func (adapter *SeekMailboxProtocolAdapter) GetRequestPID() pb.PID {
	return pb.PID_SEEK_MAILBOX_REQ
}

func (adapter *SeekMailboxProtocolAdapter) GetResponsePID() pb.PID {
	return pb.PID_SEEK_MAILBOX_RES
}

func (adapter *SeekMailboxProtocolAdapter) InitRequest(basicData *pb.BasicData, dataList ...any) error {
	request := &pb.SeekMailboxReq{
		BasicData: basicData,
	}
	adapter.protocol.ProtocolRequest = request
	return nil
}

func (adapter *SeekMailboxProtocolAdapter) InitResponse(basicData *pb.BasicData, dataList ...any) error {
	response := &pb.SeekMailboxRes{
		BasicData: basicData,
		RetCode:   dmsgProtocol.NewSuccRetCode(),
	}
	adapter.protocol.ProtocolResponse = response
	return nil
}

func (adapter *SeekMailboxProtocolAdapter) SetResponseSig(sig []byte) error {
	response, ok := adapter.protocol.ProtocolResponse.(*pb.SeekMailboxRes)
	if !ok {
		return errors.New("SeekMailboxProtocolAdapter->SetResponseSig: failed to cast request to *pb.SeekMailboxRes")
	}
	response.BasicData.Sig = sig
	return nil
}

func (adapter *SeekMailboxProtocolAdapter) CallResponseCallback(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (interface{}, error) {
	data, err := adapter.protocol.Callback.OnSeekMailboxResponse(requestProtoData, responseProtoData)
	return data, err
}

func (adapter *SeekMailboxProtocolAdapter) GetResponseBasicData() *pb.BasicData {
	response, ok := adapter.protocol.ProtocolResponse.(*pb.SeekMailboxRes)
	if !ok {
		return nil
	}
	return response.BasicData
}

func (adapter *SeekMailboxProtocolAdapter) GetResponseRetCode() *pb.RetCode {
	response, ok := adapter.protocol.ProtocolResponse.(*pb.SeekMailboxRes)
	if !ok {
		return nil
	}
	return response.RetCode
}
func (adapter *SeekMailboxProtocolAdapter) SetRequestSig(sig []byte) error {
	request, ok := adapter.protocol.ProtocolRequest.(*pb.SeekMailboxReq)
	if !ok {
		return errors.New("failed to cast request to *pb.SeekMailboxReq")
	}
	request.BasicData.Sig = sig
	return nil
}

func NewSeekMailboxProtocol(ctx context.Context, host host.Host, protocolCallback common.PubsubProtocolCallback, dmsgService common.ProtocolService) *common.PubsubProtocol {
	adapter := NewSeekMailboxProtocolAdapter()
	protocol := common.NewPubsubProtocol(ctx, host, protocolCallback, dmsgService, adapter)
	adapter.protocol = protocol
	adapter.init()
	return protocol
}
