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

func (adapter *CommonProtocolAdapter) InitRequest(basicData *pb.BasicData, dataList ...any) error {
	dmsgLog.Logger.Debugf("CommonProtocolAdapter->InitRequest: not implemented")
	return nil
}

func (adapter *CommonProtocolAdapter) InitResponse(basicData *pb.BasicData, dataList ...any) error {
	dmsgLog.Logger.Debugf("CommonProtocolAdapter->InitResponse: not implemented")
	return nil
}

func (adapter *CommonProtocolAdapter) SetResponseSig(sig []byte) error {
	dmsgLog.Logger.Debugf("CommonProtocolAdapter->SetResponseSig: not implemented")
	return nil
}

func (adapter *CommonProtocolAdapter) CallRequestCallback() (interface{}, error) {
	dmsgLog.Logger.Debugf("CommonProtocolAdapter->CallRequestCallback: not implemented")
	return nil, fmt.Errorf("CommonProtocolAdapter->CallRequestCallback: not implemented")
}

func (adapter *CommonProtocolAdapter) CallResponseCallback(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (interface{}, error) {
	dmsgLog.Logger.Debugf("CommonProtocolAdapter->CallResponseCallback: not implemented")
	return nil, fmt.Errorf("CommonProtocolAdapter->CallResponseCallback: not implemented")
}

func (adapter *CommonProtocolAdapter) GetRequestBasicData() *pb.BasicData {
	dmsgLog.Logger.Debugf("CommonProtocolAdapter->GetRequestBasicData: not implemented")
	return nil
}

func (adapter *CommonProtocolAdapter) GetResponseBasicData() *pb.BasicData {
	dmsgLog.Logger.Debugf("CommonProtocolAdapter->GetResponseBasicData: not implemented")
	return nil
}

func (adapter *CommonProtocolAdapter) GetResponseRetCode() *pb.RetCode {
	dmsgLog.Logger.Debugf("CommonProtocolAdapter->GetResponseRetCode: not implemented")
	return nil
}

func (adapter *CommonProtocolAdapter) SetRequestSig(sig []byte) error {
	dmsgLog.Logger.Debugf("CommonProtocolAdapter->SetRequestSig: not implemented")
	return nil
}
