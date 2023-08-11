package adapter

import (
	"context"
	"errors"

	"github.com/libp2p/go-libp2p/core/host"

	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type QueryPeerProtocolAdapter struct {
	CommonProtocolAdapter
	protocol *dmsgProtocol.PubsubProtocol
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
	requestProtoMsg := &pb.QueryPeerReq{
		BasicData: basicData,
	}
	return requestProtoMsg, nil
}

func (adapter *QueryPeerProtocolAdapter) InitResponse(
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
	response := &pb.QueryPeerRes{
		BasicData: basicData,
		RetCode:   retCode,
	}

	if len(dataList) > 0 {
		peerID, ok := dataList[0].(string)
		if !ok {
			return response, errors.New("QueryPeerProtocolAdapter->InitRequest: failed to cast datalist[0] to []byte for content")
		}
		response.PeerID = peerID
	} else {
		return response, errors.New("QueryPeerProtocolAdapter->InitRequest: parameter dataList need contain content")
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

func (adapter *QueryPeerProtocolAdapter) SetResponseRetCode(
	responseProtoMsg protoreflect.ProtoMessage,
	code int32,
	result string) {
	request, ok := responseProtoMsg.(*pb.QueryPeerRes)
	if !ok {
		return
	}
	request.RetCode = dmsgProtocol.NewRetCode(code, result)
}

func (adapter *QueryPeerProtocolAdapter) SetRequestSig(
	requestProtoMsg protoreflect.ProtoMessage,
	sig []byte) error {
	request, ok := requestProtoMsg.(*pb.QueryPeerReq)
	if !ok {
		return errors.New("QueryPeerProtocolAdapter-> SetRequestSig: failed to cast request to *pb.QueryPeerReq")
	}
	request.BasicData.Sig = sig
	return nil
}

func (adapter *QueryPeerProtocolAdapter) SetResponseSig(
	responseProtoMsg protoreflect.ProtoMessage,
	sig []byte) error {
	response, ok := responseProtoMsg.(*pb.QueryPeerRes)
	if !ok {
		return errors.New("QueryPeerProtocolAdapter->SetResponseSig: failed to cast request to *pb.QueryPeerRes")
	}
	response.BasicData.Sig = sig
	return nil
}

func (adapter *QueryPeerProtocolAdapter) CallResquestCallback(
	requestProtoData protoreflect.ProtoMessage) (any, any, error) {
	data, retCode, err := adapter.protocol.Callback.OnQueryPeerRequest(requestProtoData)
	return data, retCode, err
}

func (adapter *QueryPeerProtocolAdapter) CallResponseCallback(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	data, err := adapter.protocol.Callback.OnQueryPeerResponse(requestProtoData, responseProtoData)
	return data, err
}

func NewQueryPeerProtocol(
	ctx context.Context,
	host host.Host,
	callback dmsgProtocol.PubsubProtocolCallback,
	service dmsgProtocol.ProtocolService) *dmsgProtocol.PubsubProtocol {
	adapter := NewQueryPeerProtocolAdapter()
	protocol := dmsgProtocol.NewPubsubProtocol(ctx, host, callback, service, adapter)
	adapter.protocol = protocol
	return protocol
}
