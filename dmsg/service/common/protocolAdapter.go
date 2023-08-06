package common

import (
	"github.com/libp2p/go-libp2p/core/protocol"
	dmsgLog "github.com/tinyverse-web3/tvbase/dmsg/common/log"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
)

type CommonProtocolAdapter struct {
}

func (adapter *CommonProtocolAdapter) GetResponsePID() pb.PID {
	dmsgLog.Logger.Debugf("CommonProtocolAdapter->GetRequestPID: not implemented")
	return -1
}

func (adapter *CommonProtocolAdapter) GetStreamResponsePID() protocol.ID {
	dmsgLog.Logger.Debugf("CommonProtocolAdapter->GetStreamResponsePID: not implemented")
	return protocol.ID("noimplement")
}

func (adapter *CommonProtocolAdapter) GetStreamRequestPID() protocol.ID {
	dmsgLog.Logger.Debugf("CommonProtocolAdapter->GetStreamRequestPID: not implemented")
	return protocol.ID("noimplement")
}

func (adapter *CommonProtocolAdapter) SetProtocolResponseFailRet(errMsg string) {
	dmsgLog.Logger.Debugf("CommonProtocolAdapter->SetProtocolResponseFailRet: not implemented")
}

func (adapter *CommonProtocolAdapter) SetProtocolResponseRet(code int32, result string) {
	dmsgLog.Logger.Debugf("CommonProtocolAdapter->SetProtocolResponseRet: not implemented")
}

func (adapter *CommonProtocolAdapter) GetRequestBasicData() *pb.BasicData {
	dmsgLog.Logger.Debugf("CommonProtocolAdapter->GetRequestBasicData: not implemented")
	return nil
}

func (adapter *CommonProtocolAdapter) GetResponseBasicData() *pb.BasicData {
	dmsgLog.Logger.Debugf("CommonProtocolAdapter->GetResponseBasicData: not implemented")
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

func (adapter *CommonProtocolAdapter) CallRequestCallback() (any, any, error) {
	dmsgLog.Logger.Debugf("CommonProtocolAdapter->CallRequestCallback: not implemented")
	return nil, nil, nil
}

func (adapter *CommonProtocolAdapter) CallResponseCallback() (any, error) {
	dmsgLog.Logger.Debugf("CommonProtocolAdapter->CallResponseCallback: not implemented")
	return nil, nil
}
