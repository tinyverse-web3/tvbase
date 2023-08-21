package common

import (
	"fmt"

	"github.com/libp2p/go-libp2p/core/protocol"
	dmsgLog "github.com/tinyverse-web3/tvbase/dmsg/common/log"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type CommonProtocolAdapter struct {
}

func (adapter *CommonProtocolAdapter) GetRequestPID() pb.PID {
	dmsgLog.Logger.Debugf("CommonProtocolAdapter->GetRequestPID: not implemented")
	return -1
}

func (adapter *CommonProtocolAdapter) GetResponsePID() pb.PID {
	dmsgLog.Logger.Debugf("CommonProtocolAdapter->GetResponsePID: not implemented")
	return -1
}

func (adapter *CommonProtocolAdapter) GetStreamRequestPID() protocol.ID {
	dmsgLog.Logger.Debugf("CommonProtocolAdapter->GetStreamRequestPID: not implemented")
	return protocol.ID("noimplement")
}

func (adapter *CommonProtocolAdapter) GetStreamResponsePID() protocol.ID {
	dmsgLog.Logger.Debugf("CommonProtocolAdapter->GetStreamResponsePID: not implemented")
	return protocol.ID("noimplement")
}

func (adapter *CommonProtocolAdapter) GetEmptyRequest() protoreflect.ProtoMessage {
	dmsgLog.Logger.Debugf("CommonProtocolAdapter->GetEmptyRequest: not implemented")
	return nil
}
func (adapter *CommonProtocolAdapter) GetEmptyResponse() protoreflect.ProtoMessage {
	dmsgLog.Logger.Debugf("CommonProtocolAdapter->GetEmptyResponse: not implemented")
	return nil
}

func (adapter *CommonProtocolAdapter) GetRequestBasicData(requestProtoData protoreflect.ProtoMessage) *pb.BasicData {
	dmsgLog.Logger.Debugf("CommonProtocolAdapter->GetRequestBasicData: not implemented")
	return nil
}

func (adapter *CommonProtocolAdapter) GetResponseBasicData(responseProtoData protoreflect.ProtoMessage) *pb.BasicData {
	dmsgLog.Logger.Debugf("CommonProtocolAdapter->GetResponseBasicData: not implemented")
	return nil
}

func (adapter *CommonProtocolAdapter) InitResponse(
	requestProtoData protoreflect.ProtoMessage,
	basicData *pb.BasicData,
	dataList ...any) (protoreflect.ProtoMessage, error) {
	dmsgLog.Logger.Debugf("CommonProtocolAdapter->InitResponse: not implemented")
	return nil, fmt.Errorf("CommonProtocolAdapter->InitResponse: not implemented")
}

func (adapter *CommonProtocolAdapter) SetResponseSig(responseProtoData protoreflect.ProtoMessage, sig []byte) error {
	dmsgLog.Logger.Debugf("CommonProtocolAdapter->SetResponseSig: not implemented")
	return fmt.Errorf("CommonProtocolAdapter->SetResponseSig: not implemented")
}

func (adapter *CommonProtocolAdapter) CallRequestCallback(requestProtoData protoreflect.ProtoMessage) (any, any, error) {
	dmsgLog.Logger.Debugf("CommonProtocolAdapter->CallRequestCallback: not implemented")
	return nil, nil, fmt.Errorf("CommonProtocolAdapter->CallRequestCallback: not implemented")
}

func (adapter *CommonProtocolAdapter) CallResponseCallback(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	dmsgLog.Logger.Debugf("CommonProtocolAdapter->CallResponseCallback: not implemented")
	return nil, fmt.Errorf("CommonProtocolAdapter->CallResponseCallback: not implemented")
}
