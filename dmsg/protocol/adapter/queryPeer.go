package adapter

import (
	"context"
	"errors"

	"github.com/libp2p/go-libp2p/core/host"

	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/basic"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/common"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/newProtocol"

	"google.golang.org/protobuf/reflect/protoreflect"
)

type QueryPeerProtocolAdapter struct {
	AbstructProtocolAdapter
	protocol *basic.QueryPeerProtocol
}

func NewQueryPeerProtocolAdapter() *QueryPeerProtocolAdapter {
	ret := &QueryPeerProtocolAdapter{}
	return ret
}

func (adapter *QueryPeerProtocolAdapter) GetRequestPID() pb.PID {
	return pb.PID_QUERY_PEER_REQ
}

func (adapter *QueryPeerProtocolAdapter) GetResponsePID() pb.PID {
	return pb.PID_QUERY_PEER_RES
}

func (adapter *QueryPeerProtocolAdapter) GetEmptyRequest() protoreflect.ProtoMessage {
	return &pb.QueryPeerReq{}
}
func (adapter *QueryPeerProtocolAdapter) GetEmptyResponse() protoreflect.ProtoMessage {
	return &pb.QueryPeerRes{}
}

func (adapter *QueryPeerProtocolAdapter) InitRequest(
	basicData *pb.BasicData,
	dataList ...any) (protoreflect.ProtoMessage, error) {
	request := &pb.QueryPeerReq{
		BasicData: basicData,
	}

	if len(dataList) == 2 {
		pid, ok := dataList[1].(string)
		if !ok {
			return request, errors.New("QueryPeerProtocolAdapter->InitRequest: failed to cast datalist[1] to string for pid")
		}
		request.Pid = pid
	} else {
		return request, errors.New("QueryPeerProtocolAdapter->InitRequest: parameter dataList need contain pid")
	}

	return request, nil
}

func (adapter *QueryPeerProtocolAdapter) InitResponse(
	requestProtoData protoreflect.ProtoMessage,
	basicData *pb.BasicData,
	dataList ...any) (protoreflect.ProtoMessage, error) {
	retCode, err := getRetCode(dataList)
	if err != nil {
		return nil, err
	}
	response := &pb.QueryPeerRes{
		BasicData: basicData,
		RetCode:   retCode,
	}

	return response, nil
}

func (adapter *QueryPeerProtocolAdapter) CallRequestCallback(
	requestProtoData protoreflect.ProtoMessage) (any, any, bool, error) {
	data, retCode, abort, err := adapter.protocol.Callback.OnQueryPeerRequest(requestProtoData)
	return data, retCode, abort, err
}

func (adapter *QueryPeerProtocolAdapter) CallResponseCallback(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	return nil, nil
}

func NewQueryPeerProtocol(
	ctx context.Context,
	host host.Host,
	callback common.QueryPeerCallback,
	service common.DmsgService,
) *basic.QueryPeerProtocol {
	adapter := NewQueryPeerProtocolAdapter()
	protocol := newProtocol.NewQueryPeerProtocol(ctx, host, callback, service, adapter)
	adapter.protocol = protocol
	return protocol
}
