package tvbase

import (
	"fmt"

	tvLog "github.com/tinyverse-web3/tvbase/common/log"
	tvProtocol "github.com/tinyverse-web3/tvbase/common/protocol"
	dmsgClient "github.com/tinyverse-web3/tvbase/dmsg/client"
	customProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol/custom"
	dmsgService "github.com/tinyverse-web3/tvbase/dmsg/service"
)

func (m *Tvbase) initProtocol() error {
	m.nodeInfoService = tvProtocol.NewNodeInfoService(m.host, m.nodeCfg)
	return nil
}

// regist custom stream client protocol
func (m *Tvbase) RegistCSCProtocol(protocol customProtocol.CustomProtocolClient) error {
	if m.DmsgService == nil {
		tvLog.Logger.Errorf("Infrasture->RegistCSCProtocol: m.DmsgService is nil")
		return fmt.Errorf("Infrasture->RegistCSCProtocol: m.DmsgService is nil")
	}
	service, ok := m.DmsgService.(*dmsgClient.DmsgService)
	if !ok {
		tvLog.Logger.Errorf("Infrasture->RegistCSCProtocol: m.DmsgService is not ClientDmsgService")
		return fmt.Errorf("Infrasture->RegistCSCProtocol: m.DmsgService is not ClientDmsgService")
	}
	service.RegistCustomStreamProtocol(protocol)
	return nil
}

// regist custom stream service protocol
func (m *Tvbase) RegistCSSProtocol(protocolService customProtocol.CustomProtocolService) error {
	if m.DmsgService == nil {
		tvLog.Logger.Errorf("Infrasture->RegistCSCProtocol: m.DmsgService is nil")
		return fmt.Errorf("Infrasture->RegistCSCProtocol: m.DmsgService is nil")
	}
	dmsgService, ok := m.DmsgService.(*dmsgService.DmsgService)
	if !ok {
		tvLog.Logger.Errorf("Infrasture->RegistCSCProtocol: m.DmsgService is not ServiceDmsgService")
		return fmt.Errorf("Infrasture->RegistCSCProtocol: m.DmsgService is not ServiceDmsgService")
	}
	dmsgService.RegistCustomStreamProtocol(protocolService)
	return nil
}
