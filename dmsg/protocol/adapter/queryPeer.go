package adapter

import (
	"context"
	"errors"
	"fmt"

	"github.com/libp2p/go-libp2p/core/host"

	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type QueryPeerProtocolAdapter struct {
	CommonProtocolAdapter
	protocol *dmsgProtocol.QueryPeerProtocol
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
	retCode, err := GetRetCode(dataList)
	if err != nil {
		return nil, err
	}
	response := &pb.QueryPeerRes{
		BasicData: basicData,
		RetCode:   retCode,
	}

	return response, nil
}

func (adapter *QueryPeerProtocolAdapter) GetRequestBasicData(
	requestProtoMsg protoreflect.ProtoMessage) *pb.BasicData {
	request, ok := requestProtoMsg.(*pb.QueryPeerReq)
	if !ok {
		return nil
	}
	return request.BasicData
}

func (adapter *QueryPeerProtocolAdapter) GetResponseBasicData(
	responseProtoMsg protoreflect.ProtoMessage) *pb.BasicData {
	response, ok := responseProtoMsg.(*pb.QueryPeerRes)
	if !ok {
		return nil
	}
	return response.BasicData
}

func (adapter *QueryPeerProtocolAdapter) GetResponseRetCode(
	responseProtoMsg protoreflect.ProtoMessage) *pb.RetCode {
	response, ok := responseProtoMsg.(*pb.QueryPeerRes)
	if !ok {
		return nil
	}
	return response.RetCode
}

func (adapter *QueryPeerProtocolAdapter) SetRequestSig(
	requestProtoMsg protoreflect.ProtoMessage,
	sig []byte) error {
	request, ok := requestProtoMsg.(*pb.QueryPeerReq)
	if !ok {
		return fmt.Errorf("QueryPeerProtocolAdapter->SetRequestSig: failed to cast request to *pb.QueryPeerReq")
	}
	request.BasicData.Sig = sig
	return nil
}

func (adapter *QueryPeerProtocolAdapter) SetResponseSig(
	responseProtoMsg protoreflect.ProtoMessage,
	sig []byte) error {
	response, ok := responseProtoMsg.(*pb.QueryPeerRes)
	if !ok {
		return fmt.Errorf("QueryPeerProtocolAdapter->SetResponseSig: failed to cast request to *pb.QueryPeerRes")
	}
	response.BasicData.Sig = sig
	return nil
}

func (adapter *QueryPeerProtocolAdapter) SetResponseRetCode(
	responseProtoMsg protoreflect.ProtoMessage,
	code int32,
	result string) {
	response, ok := responseProtoMsg.(*pb.QueryPeerRes)
	if !ok {
		return
	}
	response.RetCode = dmsgProtocol.NewRetCode(code, result)
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
	callback dmsgProtocol.QueryPeerCallback,
	service dmsgProtocol.DmsgService,
) *dmsgProtocol.QueryPeerProtocol {
	adapter := NewQueryPeerProtocolAdapter()
	protocol := dmsgProtocol.NewQueryPeerProtocol(ctx, host, callback, service, adapter)
	adapter.protocol = protocol
	return protocol
}
