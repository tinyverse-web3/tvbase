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

type QueryPeerProtocolAdapter struct {
	common.CommonProtocolAdapter
	protocol *common.PubsubProtocol
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
	basicData *pb.BasicData,
	dataList ...any) (protoreflect.ProtoMessage, error) {
	var responseProtoMsg *pb.QueryPeerRes
	if len(dataList) == 1 {
		peerID, ok := dataList[0].(string)
		if !ok {
			return nil, errors.New("SendMsgProtocolAdapter->InitRequest: failed to cast datalist[0] to []byte for content")
		}
		responseProtoMsg = &pb.QueryPeerRes{
			BasicData: basicData,
			PeerID:    peerID,
			RetCode:   dmsgProtocol.NewSuccRetCode(),
		}
	} else {
		return nil, errors.New("SendMsgProtocolAdapter->InitRequest: parameter dataList need contain content")
	}
	return responseProtoMsg, nil
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
	requestProtoData protoreflect.ProtoMessage) (interface{}, error) {
	data, err := adapter.protocol.Callback.OnQueryPeerRequest(requestProtoData)
	return data, err
}

func (adapter *QueryPeerProtocolAdapter) CallResponseCallback(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (interface{}, error) {
	data, err := adapter.protocol.Callback.OnQueryPeerResponse(requestProtoData, responseProtoData)
	return data, err
}

func NewQueryPeerProtocol(ctx context.Context, host host.Host, protocolCallback common.PubsubProtocolCallback, dmsgService common.ProtocolService) *common.PubsubProtocol {
	adapter := NewQueryPeerProtocolAdapter()
	protocol := common.NewPubsubProtocol(ctx, host, protocolCallback, dmsgService, adapter)
	adapter.protocol = protocol
	return protocol
}
