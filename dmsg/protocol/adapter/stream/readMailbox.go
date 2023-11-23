package stream

import (
	"context"
	"errors"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	basicAdapter "github.com/tinyverse-web3/tvbase/dmsg/protocol/adapter/basic"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/basic"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/common"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/newProtocol"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type ReadMailRequestParam struct {
	ItemList  []*pb.MailboxItem
	ExistData bool
}

type ReadMailboxProtocolAdapter struct {
	basicAdapter.AbstructProtocolAdapter
	protocol *basic.MailboxSProtocol
}

func NewReadMailboxMsgProtocolAdapter() *ReadMailboxProtocolAdapter {
	ret := &ReadMailboxProtocolAdapter{}
	return ret
}

func (adapter *ReadMailboxProtocolAdapter) GetRequestPID() pb.PID {
	return pb.PID_READ_MAILBOX_REQ
}

func (adapter *ReadMailboxProtocolAdapter) GetResponsePID() pb.PID {
	return pb.PID_READ_MAILBOX_RES
}

func (adapter *ReadMailboxProtocolAdapter) GetStreamRequestPID() protocol.ID {
	return common.PidReadMailboxMsgReq
}

func (adapter *ReadMailboxProtocolAdapter) GetStreamResponsePID() protocol.ID {
	return common.PidReadMailboxMsgRes
}

func (adapter *ReadMailboxProtocolAdapter) GetEmptyRequest() protoreflect.ProtoMessage {
	return &pb.ReadMailboxReq{}
}
func (adapter *ReadMailboxProtocolAdapter) GetEmptyResponse() protoreflect.ProtoMessage {
	return &pb.ReadMailboxRes{}
}

func (adapter *ReadMailboxProtocolAdapter) InitRequest(
	basicData *pb.BasicData,
	dataList ...any) (protoreflect.ProtoMessage, error) {
	request := &pb.ReadMailboxReq{
		BasicData: basicData,
	}

	if len(dataList) == 1 {
		clearMode, ok := dataList[0].(bool)
		if !ok {
			return request, errors.New("ReadMailboxMsgProtocolAdapter->InitRequest: failed to cast datalist[0] to bool for clearMode")
		}
		request.ClearMode = clearMode
	} else {
		return request, errors.New("ReadMailboxMsgProtocolAdapter->InitRequest: parameter dataList need contain clearMode")
	}

	return request, nil
}

func (adapter *ReadMailboxProtocolAdapter) InitResponse(
	requestProtoData protoreflect.ProtoMessage,
	basicData *pb.BasicData,
	dataList ...any) (protoreflect.ProtoMessage, error) {
	retCode, err := basicAdapter.GetRetCode(dataList)
	if err != nil {
		return nil, err
	}
	response := &pb.ReadMailboxRes{
		BasicData: basicData,
		RetCode:   retCode,
	}

	if len(dataList) < 1 {
		return nil, errors.New("ReadMailboxMsgProtocolAdapter->InitResponse: dataList need contain MailboxItemList")
	}
	requestParam, ok := dataList[0].(*ReadMailRequestParam)
	if !ok {
		return response, errors.New("ReadMailboxMsgProtocolAdapter->InitResponse: fail to cast dataList[0] to []*pb.MailboxMsgData")
	}

	response.ContentList = requestParam.ItemList
	response.ExistData = requestParam.ExistData
	return response, nil
}

func (adapter *ReadMailboxProtocolAdapter) CallRequestCallback(
	requestProtoData protoreflect.ProtoMessage) (any, any, bool, error) {
	data, retCode, abort, err := adapter.protocol.Callback.OnReadMailboxRequest(requestProtoData)
	return data, retCode, abort, err
}

func (adapter *ReadMailboxProtocolAdapter) CallResponseCallback(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	data, err := adapter.protocol.Callback.OnReadMailboxResponse(requestProtoData, responseProtoData)
	return data, err
}

func NewReadMailboxMsgProtocol(
	ctx context.Context,
	host host.Host,
	callback common.MailboxSpCallback,
	service common.DmsgService,
	enableRequest bool,
	pubkey string,
) *basic.MailboxSProtocol {
	adapter := NewReadMailboxMsgProtocolAdapter()
	protocol := newProtocol.NewMailboxSProtocol(ctx, host, callback, service, adapter, enableRequest, pubkey)
	adapter.protocol = protocol
	return protocol
}
