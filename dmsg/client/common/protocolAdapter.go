package common

import (
	"fmt"

	"github.com/libp2p/go-libp2p/core/protocol"
	dmsgLog "github.com/tinyverse-web3/tvbase/dmsg/common/log"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
)

type CommonProtocolAdapter struct {
}

func (adapter *CommonProtocolAdapter) GetRequestProtocolID() pb.ProtocolID {
	dmsgLog.Logger.Warnf("CommonProtocolAdapter->GetRequestProtocolID: not implemented")
	return -1
}

func (adapter *CommonProtocolAdapter) GetStreamRequestProtocolID() protocol.ID {
	dmsgLog.Logger.Warnf("CommonProtocolAdapter->GetStreamRequestProtocolID: not implemented")
	return protocol.ID("noimplement")
}

func (adapter *CommonProtocolAdapter) InitProtocolRequest(basicData *pb.BasicData, dataList ...any) error {
	dmsgLog.Logger.Warnf("CommonProtocolAdapter->InitProtocolRequest: not implemented")
	return nil
}

func (adapter *CommonProtocolAdapter) SetCustomContent(protocolId string, requestContent []byte) error {
	dmsgLog.Logger.Warnf("CommonProtocolAdapter->SetCustomContent: not implemented")
	return fmt.Errorf("CommonProtocolAdapter->SetCustomContent: not implemented")
}

func (adapter *CommonProtocolAdapter) CallProtocolResponseCallback() (interface{}, error) {
	dmsgLog.Logger.Warnf("CommonProtocolAdapter->CallProtocolResponseCallback: not implemented")
	return nil, fmt.Errorf("CommonProtocolAdapter->CallProtocolResponseCallback: not implemented")
}

func (adapter *CommonProtocolAdapter) GetProtocolResponseBasicData() *pb.BasicData {
	dmsgLog.Logger.Warnf("CommonProtocolAdapter->GetProtocolResponseBasicData: not implemented")
	return nil
}

func (adapter *CommonProtocolAdapter) GetProtocolResponseRetCode() *pb.RetCode {
	dmsgLog.Logger.Warnf("CommonProtocolAdapter->GetProtocolResponseRetCode: not implemented")
	return nil
}

func (adapter *CommonProtocolAdapter) SetProtocolRequestSign(signature []byte) {
	dmsgLog.Logger.Warnf("CommonProtocolAdapter->SetProtocolRequestSign: not implemented")
}
