package adapter

import (
	"fmt"

	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
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

func getRetCode(dataList ...any) (*pb.RetCode, error) {
	retCode := dmsgProtocol.NewSuccRetCode()
	if len(dataList) > 1 && dataList[1] != nil {
		data, ok := dataList[1].(*pb.RetCode)
		if !ok {
			return nil, fmt.Errorf("getRetCode: fail to cast dataList[1] to *pb.RetCode")
		} else {
			if data == nil {
				fmt.Printf("getRetCode: data == nil")
				return nil, fmt.Errorf("getRetCode: data == nil")
			}
			retCode = data
		}
	}
	return retCode, nil
}
