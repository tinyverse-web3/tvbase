package adapter

import (
	"context"
	"errors"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type ReadMailRequestParam struct {
	ItemList  []*pb.MailboxItem
	ExistData bool
}

type ReadMailboxMsgProtocolAdapter struct {
	CommonProtocolAdapter
	protocol *dmsgProtocol.MailboxSProtocol
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

func (adapter *ReadMailboxMsgProtocolAdapter) InitResponse(
	requestProtoData protoreflect.ProtoMessage,
	basicData *pb.BasicData,
	dataList ...any) (protoreflect.ProtoMessage, error) {
	retCode, err := getRetCode(dataList)
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

func (adapter *ReadMailboxMsgProtocolAdapter) CallRequestCallback(
	requestProtoData protoreflect.ProtoMessage) (any, any, bool, error) {
	data, retCode, abort, err := adapter.protocol.Callback.OnReadMailboxMsgRequest(requestProtoData)
	return data, retCode, abort, err
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
	callback dmsgProtocol.MailboxSpCallback,
	service dmsgProtocol.DmsgService,
	enableRequest bool,
) *dmsgProtocol.MailboxSProtocol {
	adapter := NewReadMailboxMsgProtocolAdapter()
	protocol := dmsgProtocol.NewMailboxSProtocol(ctx, host, callback, service, adapter, enableRequest)
	adapter.protocol = protocol
	return protocol
}
