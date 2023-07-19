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
	anyConnected := false
	tvLog.Logger.Info("tvBase->DiscoverRendezvousPeers: Searching for rendezvous peers...")
	for !anyConnected {
		rendezvousPeerCount := 0
		start := time.Now()
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
				continue
			}

			rendezvousPeerCount++
			anyConnected = true
			tvLog.Logger.Debugf("tvBase->DiscoverRendezvousPeers: It took %v seconds succcess connect to the rendezvous peer:%v",
				time.Since(start).Seconds(), peer.ID.Pretty())

			go m.registPeerInfo(peer.ID)
		}

		if rendezvousPeerCount == 0 {
			tvLog.Logger.Infof("tvBase->DiscoverRendezvousPeers: The number of peers is equal to 0, wait 10 second to search again")
			time.Sleep(10 * time.Second)
		} else {
			tvLog.Logger.Infof("tvBase->DiscoverRendezvousPeers: The number of rendezvous peer is %v", rendezvousPeerCount)
		}
	}
}
