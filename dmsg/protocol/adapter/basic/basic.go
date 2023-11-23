package basic

import (
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type AbstructProtocolAdapter struct {
}

func (adapter *AbstructProtocolAdapter) GetRequestPID() pb.PID {
	return -1
}

func (adapter *AbstructProtocolAdapter) GetResponsePID() pb.PID {
	return -1
}

func (adapter *AbstructProtocolAdapter) GetStreamRequestPID() protocol.ID {
	return protocol.ID("unknown")
}

func (adapter *AbstructProtocolAdapter) GetStreamResponsePID() protocol.ID {
	return protocol.ID("unknown")
}

func (adapter *AbstructProtocolAdapter) GetEmptyRequest() protoreflect.ProtoMessage {
	return nil
}
func (adapter *AbstructProtocolAdapter) GetEmptyResponse() protoreflect.ProtoMessage {
	return nil
}

func (adapter *AbstructProtocolAdapter) InitRequest(
	basicData *pb.BasicData,
	dataList ...any) (protoreflect.ProtoMessage, error) {
	return nil, nil
}

func (adapter *AbstructProtocolAdapter) InitResponse(
	requestProtoData protoreflect.ProtoMessage,
	basicData *pb.BasicData,
	dataList ...any) (protoreflect.ProtoMessage, error) {
	return nil, nil
}

func (adapter *AbstructProtocolAdapter) CallRequestCallback(
	requestProtoData protoreflect.ProtoMessage) (any, any, error) {
	return nil, nil, nil
}

func (adapter *AbstructProtocolAdapter) CallResponseCallback(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	return nil, nil
}
