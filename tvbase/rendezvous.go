package tvbase

import (
	"time"

	drouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	tvLog "github.com/tinyverse-web3/tvbase/common/log"
	tvUtil "github.com/tinyverse-web3/tvbase/common/util"
)

const TinverseInfrastureRendezvous = "tinverseInfrasture/discover-rendzvous/common"

func (m *TvBase) initRendezvous() error {
	var err error
	m.pubRoutingDiscovery = drouting.NewRoutingDiscovery(m.dht)
	tvUtil.PubsubAdvertise(m.ctx, m.pubRoutingDiscovery, TinverseInfrastureRendezvous)
	return err
}

func (m *TvBase) DiscoverRendezvousPeers() {
	start := time.Now()
	anyConnected := false
	tvLog.Logger.Info("tvBase->DiscoverRendezvousPeers: Searching for rendezvous peers...")
	for !anyConnected {
		peerChan, err := m.pubRoutingDiscovery.FindPeers(m.ctx, TinverseInfrastureRendezvous)
		if err != nil {
			tvLog.Logger.Errorf("tvBase->DiscoverRendezvousPeers: Searching rendezvous peer error: %v", err)
			continue
		}

		for peer := range peerChan {
			if peer.ID == m.host.ID() {
				continue
			}
			if len(peer.Addrs) == 0 {
				continue
			}

			err := m.host.Connect(m.ctx, peer)
			if err != nil {
				tvLog.Logger.Warnf("tvBase->DiscoverRendezvousPeers: Fail connect to the rendezvous peerID: %v, error: %v", peer.ID, err)
				addrInfo := m.host.Peerstore().PeerInfo(peer.ID)
				for _, peerAddr := range addrInfo.Addrs {
					tvLog.Logger.Errorf("tvBase->registPeerInfo: peerId addr: %v", peerAddr)
				}
				continue
			}
			anyConnected = true
			tvLog.Logger.Infof("tvBase->DiscoverRendezvousPeers: It took %v seconds succcess connect to the rendezvous peer:%v",
				time.Since(start).Seconds(), peer.ID.Pretty())

			go m.registPeerInfo(peer.ID)
		}
		if anyConnected {
			tvLog.Logger.Infof("tvBase->DiscoverRendezvousPeers: It took %v seconds connect to all rendezvous peers\n", time.Since(start).Seconds())
		}
	}
}
