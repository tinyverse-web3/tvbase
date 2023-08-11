package tvbase

import (
	"fmt"

	tvLog "github.com/tinyverse-web3/tvbase/common/log"
	tvProtocol "github.com/tinyverse-web3/tvbase/common/protocol"
	dmsgClient "github.com/tinyverse-web3/tvbase/dmsg/client"
	customProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol/custom"
	dmsgService "github.com/tinyverse-web3/tvbase/dmsg/service"
)

func (m *TvBase) initProtocol() error {
	m.nodeInfoService = tvProtocol.NewNodeInfoService(m.host, m.nodeCfg)
	return nil
}

// regist custom stream client protocol
func (m *TvBase) RegistCSCProtocol(protocol customProtocol.CustomStreamProtocolClient) error {
	if m.DmsgService == nil {
		tvLog.Logger.Errorf("tvBase->RegistCSCProtocol: m.DmsgService is nil")
		return fmt.Errorf("tvBase->RegistCSCProtocol: m.DmsgService is nil")
	}
	service, ok := m.DmsgService.(*dmsgClient.DmsgService)
	if !ok {
		tvLog.Logger.Errorf("tvBase->RegistCSCProtocol: m.DmsgService is not ClientDmsgService")
		return fmt.Errorf("tvBase->RegistCSCProtocol: m.DmsgService is not ClientDmsgService")
	}
	service.RegistCustomStreamProtocol(protocol)
	return nil
}

// regist custom stream service protocol
func (m *TvBase) RegistCSSProtocol(protocol customProtocol.CustomStreamProtocolService) error {
	if m.DmsgService == nil {
		tvLog.Logger.Errorf("tvBase->RegistCSSProtocol: m.DmsgService is nil")
		return fmt.Errorf("tvBase->RegistCSSProtocol: m.DmsgService is nil")
	}
	service, ok := m.DmsgService.(*dmsgService.DmsgService)
	if !ok {
		tvLog.Logger.Errorf("tvBase->RegistCSSProtocol: m.DmsgService is not ServiceDmsgService")
		return fmt.Errorf("tvBase->RegistCSSProtocol: m.DmsgService is not ServiceDmsgService")
	}
	service.RegistCustomStreamProtocol(protocol)
	return nil
}
