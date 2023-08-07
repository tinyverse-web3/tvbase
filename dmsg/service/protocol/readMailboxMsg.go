package protocol

import (
	"context"
	"errors"

	"github.com/libp2p/go-libp2p/core/host"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/service/common"
)

type ReadMailboxMsgProtocolAdapter struct {
	common.CommonProtocolAdapter
	protocol *common.StreamProtocol
}

func NewReadMailboxMsgProtocolAdapter() *ReadMailboxMsgProtocolAdapter {
	ret := &ReadMailboxMsgProtocolAdapter{}
	return ret
}

func (adapter *ReadMailboxMsgProtocolAdapter) init() {
}

func (adapter *ReadMailboxMsgProtocolAdapter) GetRequestPID() pb.PID {
	return pb.PID_READ_MAILBOX_REQ
}

func (adapter *ReadMailboxMsgProtocolAdapter) GetResponsePID() pb.PID {
	return pb.PID_READ_MAILBOX_RES
}

func (adapter *ReadMailboxMsgProtocolAdapter) GetStreamResponsePID() protocol.ID {
	return dmsgProtocol.PidReadMailboxMsgRes
}

func (adapter *ReadMailboxMsgProtocolAdapter) GetStreamRequestPID() protocol.ID {
	return dmsgProtocol.PidReadMailboxMsgReq
}

func (adapter *ReadMailboxMsgProtocolAdapter) GetEmptyRequest() protoreflect.ProtoMessage {
	return &pb.ReadMailboxReq{}
}
func (adapter *ReadMailboxMsgProtocolAdapter) GetEmptyResponse() protoreflect.ProtoMessage {
	return &pb.ReadMailboxRes{}
}

func (adapter *ReadMailboxMsgProtocolAdapter) GetRequestBasicData(requestProtoData protoreflect.ProtoMessage) *pb.BasicData {
	request, ok := requestProtoData.(*pb.ReadMailboxReq)
	if !ok {
		return nil
	}
	return request.BasicData
}

func (adapter *ReadMailboxMsgProtocolAdapter) GetResponseBasicData(responseProtoData protoreflect.ProtoMessage) *pb.BasicData {
	request, ok := responseProtoData.(*pb.ReadMailboxRes)
	if !ok {
		return nil
	}
	return request.BasicData
}

func (adapter *ReadMailboxMsgProtocolAdapter) InitResponse(
	requestProtoData protoreflect.ProtoMessage,
	basicData *pb.BasicData,
	dataList ...any) (protoreflect.ProtoMessage, error) {
	if len(dataList) < 1 {
		return nil, errors.New("ReadMailboxMsgProtocolAdapter->InitResponse: dataList need contain MailboxItemList")
	}
	mailboxItemList, ok := dataList[0].([]*pb.MailboxItem)
	if !ok {
		return nil, errors.New("ReadMailboxMsgProtocolAdapter->InitResponse: fail to cast dataList[0] to []*pb.MailboxMsgData")
	}

	var retCode *pb.RetCode
	if len(dataList) > 1 {
		retCode, ok = dataList[1].(*pb.RetCode)
		if !ok {
			retCode = dmsgProtocol.NewSuccRetCode()
		}
	}

	response := &pb.ReadMailboxRes{
		BasicData:   basicData,
		RetCode:     retCode,
		ContentList: mailboxItemList,
	}

	return response, nil
}

func (adapter *ReadMailboxMsgProtocolAdapter) SetResponseSig(responseProtoData protoreflect.ProtoMessage, sig []byte) error {
	response, ok := responseProtoData.(*pb.ReadMailboxRes)
	if !ok {
		return errors.New("CreateMailboxProtocolAdapter->ReadMailboxMsgProtocolAdapter: failed to cast response to *pb.ReadMailboxMsgRes")
	}
	response.BasicData.Sig = sig
	return nil
}

func (adapter *ReadMailboxMsgProtocolAdapter) CallRequestCallback(requestProtoData protoreflect.ProtoMessage) (any, any, error) {
	data, retCode, err := adapter.protocol.Callback.OnReadMailboxMsgRequest(requestProtoData)
	return data, retCode, err
}

func NewReadMailboxMsgProtocol(ctx context.Context, host host.Host, protocolService common.ProtocolService, protocolCallback common.StreamProtocolCallback) *common.StreamProtocol {
	adapter := NewReadMailboxMsgProtocolAdapter()
	protocol := common.NewStreamProtocol(ctx, host, protocolService, protocolCallback, adapter)
	adapter.protocol = protocol
	adapter.init()
	return protocol
}
