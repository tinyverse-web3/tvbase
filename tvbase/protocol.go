package tvbase

import (
	tvProtocol "github.com/tinyverse-web3/tvbase/common/protocol"
)

func (m *TvBase) initProtocol() error {
	m.nodeInfoService = tvProtocol.NewNodeInfoService(m.host, m.nodeCfg.Mode)
	return nil
}
