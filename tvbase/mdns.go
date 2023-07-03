package tvbase

import (
	libp2pPeer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	tvLog "github.com/tinyverse-web3/tvbase/common/log"
)

const MdnsServiceName = "tinverseInfrasture/mdns/default"

// for mdns Notifee interface
func (m *TvBase) HandlePeerFound(p libp2pPeer.AddrInfo) {
	if p.ID == m.host.ID() {
		return
	}
	if len(p.Addrs) == 0 {
		return
	}
	err := m.host.Connect(m.ctx, p)
	if err != nil {
		tvLog.Logger.Errorf("fail connect to mdns peer: %v, err:%v", p, err)
		return
	} else {
		tvLog.Logger.Infof("success connect to mdns peer: %v", p)
	}
	go m.registPeerInfo(p.ID)
}

func (m *TvBase) initMdns() error {
	if !m.nodeCfg.Network.EnableMdns {
		return nil
	}
	m.mdnsService = mdns.NewMdnsService(m.host, MdnsServiceName, m)
	if err := m.mdnsService.Start(); err != nil {
		tvLog.Logger.Errorf("DmsgService->enableMdns: mdns start error: %v", err)
		return err
	}
	return nil
}

func (m *TvBase) disableMdns() error {
	if m.mdnsService != nil {
		m.mdnsService.Close()
		m.mdnsService = nil
	}
	return nil
}
