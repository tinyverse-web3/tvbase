package tvbase

import (
	"context"
	"strings"

	"github.com/libp2p/go-libp2p/core/host"
	libp2pPeer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	tvLog "github.com/tinyverse-web3/tvbase/common/log"
	"go.uber.org/fx"
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

func (m *TvBase) initMdns(ph host.Host, lc fx.Lifecycle) (mdns.Service, error) {
	if !m.nodeCfg.Network.EnableMdns {
		return nil, nil
	}
	mdnsService := mdns.NewMdnsService(ph, MdnsServiceName, m)
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			err := mdnsService.Start()
			if err != nil && !strings.Contains(err.Error(), "netlinkrib") {
				tvLog.Logger.Errorf("tvBase->initMdns: mdns start error: %v", err)
				return err
			}
			tvLog.Logger.Info("tvBase->initMdns: mdns start...")
			return nil
		},
		OnStop: func(_ context.Context) error {
			err := mdnsService.Close()
			if err != nil {
				tvLog.Logger.Errorf("tvBase->initMdns: mdns close error: %v", err)
				return err
			}
			tvLog.Logger.Info("tvBase->initMdns: mdns is closed")
			return nil
		},
	})

	return mdnsService, nil
}
