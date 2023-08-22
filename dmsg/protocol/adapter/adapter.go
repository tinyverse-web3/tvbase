package adapter

import (
	"fmt"

	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type CommonProtocolAdapter struct {
}

func (adapter *CommonProtocolAdapter) GetRequestPID() pb.PID {
	return -1
}

func (adapter *CommonProtocolAdapter) GetResponsePID() pb.PID {
	return -1
}

func (adapter *CommonProtocolAdapter) GetStreamRequestPID() protocol.ID {
	return protocol.ID("unknown")
}

func (adapter *CommonProtocolAdapter) GetStreamResponsePID() protocol.ID {
	return protocol.ID("unknown")
}

func (adapter *CommonProtocolAdapter) GetEmptyRequest() protoreflect.ProtoMessage {
	return nil
}
func (adapter *CommonProtocolAdapter) GetEmptyResponse() protoreflect.ProtoMessage {
	return nil
}

func (adapter *CommonProtocolAdapter) InitRequest(
	basicData *pb.BasicData,
	dataList ...any) (protoreflect.ProtoMessage, error) {
	return nil, fmt.Errorf("CommonProtocolAdapter->InitRequest: not implemented")
}

func (adapter *CommonProtocolAdapter) InitResponse(
	requestProtoData protoreflect.ProtoMessage,
	basicData *pb.BasicData,
	dataList ...any) (protoreflect.ProtoMessage, error) {
	return nil, fmt.Errorf("CommonProtocolAdapter->InitRequest: not implemented")
}

func (adapter *CommonProtocolAdapter) GetRequestBasicData(
	requestProtoMsg protoreflect.ProtoMessage) *pb.BasicData {
	return nil
}

func (adapter *CommonProtocolAdapter) GetResponseBasicData(
	responseProtoMsg protoreflect.ProtoMessage) *pb.BasicData {
	return nil
}

func (adapter *CommonProtocolAdapter) GetResponseRetCode(
	responseProtoMsg protoreflect.ProtoMessage) *pb.RetCode {
	return nil
}

func (adapter *CommonProtocolAdapter) SetRequestSig(
	requestProtoMsg protoreflect.ProtoMessage,
	sig []byte) error {
	return nil
}

func (adapter *CommonProtocolAdapter) SetResponseSig(
	responseProtoMsg protoreflect.ProtoMessage,
	sig []byte) error {
	return nil
}

func (adapter *CommonProtocolAdapter) CallRequestCallback(
	requestProtoData protoreflect.ProtoMessage) (any, any, error) {
	return nil, nil, fmt.Errorf("CommonProtocolAdapter->CallRequestCallback: not implemented")
}

func (adapter *CommonProtocolAdapter) CallResponseCallback(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	return nil, fmt.Errorf("CommonProtocolAdapter->CallResponseCallback: not implemented")
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
