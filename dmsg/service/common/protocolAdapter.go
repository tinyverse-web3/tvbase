package common

import (
	"github.com/libp2p/go-libp2p/core/protocol"
	dmsgLog "github.com/tinyverse-web3/tvbase/dmsg/common/log"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
)

type CommonProtocolAdapter struct {
}

func (adapter *CommonProtocolAdapter) GetResponseProtocolID() pb.ProtocolID {
	dmsgLog.Logger.Debugf("CommonProtocolAdapter->GetRequestProtocolID: not implemented")
	return -1
}

func (adapter *CommonProtocolAdapter) GetStreamResponseProtocolID() protocol.ID {
	dmsgLog.Logger.Debugf("CommonProtocolAdapter->GetStreamResponseProtocolID: not implemented")
	return protocol.ID("noimplement")
}

func (adapter *CommonProtocolAdapter) SetProtocolResponseFailRet(errMsg string) {
	dmsgLog.Logger.Debugf("CommonProtocolAdapter->SetProtocolResponseFailRet: not implemented")
}

func (adapter *CommonProtocolAdapter) SetProtocolResponseRet(code int32, result string) {
	dmsgLog.Logger.Debugf("CommonProtocolAdapter->SetProtocolResponseRet: not implemented")
}

func (adapter *CommonProtocolAdapter) GetProtocolRequestBasicData() *pb.BasicData {
	dmsgLog.Logger.Debugf("CommonProtocolAdapter->GetProtocolRequestBasicData: not implemented")
	return nil
}

func (adapter *CommonProtocolAdapter) GetProtocolResponseBasicData() *pb.BasicData {
	dmsgLog.Logger.Debugf("CommonProtocolAdapter->GetProtocolResponseBasicData: not implemented")
	return nil
}

func (adapter *CommonProtocolAdapter) InitProtocolResponse(basicData *pb.BasicData, data interface{}) error {
	dmsgLog.Logger.Debugf("CommonProtocolAdapter->InitProtocolResponse: not implemented")
	return nil
}

func (adapter *CommonProtocolAdapter) SetProtocolResponseSign(signature []byte) error {
	dmsgLog.Logger.Debugf("CommonProtocolAdapter->SetProtocolResponseSign: not implemented")
	return nil
}

func (adapter *CommonProtocolAdapter) CallProtocolRequestCallback() (interface{}, error) {
	dmsgLog.Logger.Debugf("CommonProtocolAdapter->CallProtocolRequestCallback: not implemented")
	return nil, nil
}

func (adapter *CommonProtocolAdapter) CallProtocolResponseCallback() (interface{}, error) {
	dmsgLog.Logger.Debugf("CommonProtocolAdapter->CallProtocolResponseCallback: not implemented")
	return nil, nil
}
