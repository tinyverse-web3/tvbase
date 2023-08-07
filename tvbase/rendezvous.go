package tvbase

import (
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	drouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	tvLog "github.com/tinyverse-web3/tvbase/common/log"
	tvPeer "github.com/tinyverse-web3/tvbase/common/peer"
	tvUtil "github.com/tinyverse-web3/tvbase/common/util"
)

const TvbaseRendezvous = "tvbase/discover-rendzvous/common"

func (m *TvBase) initRendezvous() error {
	if m.pubRoutingDiscovery == nil {
		m.rendezvousCbList = make([]tvPeer.RendezvousCallback, 0)
		m.pubRoutingDiscovery = drouting.NewRoutingDiscovery(m.dht)
		tvUtil.PubsubAdvertise(m.ctx, m.pubRoutingDiscovery, TvbaseRendezvous)

		handleNoNet := func(peerID peer.ID) error {
			if !m.IsExistConnectedPeer() {
				m.IsRendezvous = false
			}
			return nil
		}

		m.RegistNotConnectedCallback(handleNoNet)
	}
	return nil
}

func (m *TvBase) RegistRendezvousCallback(callback tvPeer.RendezvousCallback) {
	m.rendezvousCbList = append(m.rendezvousCbList, callback)
}

func (m *TvBase) DiscoverRendezvousPeers() {
	// anyConnected := false
	tvLog.Logger.Info("tvBase->DiscoverRendezvousPeers: Searching for rendezvous peers...")
	for !m.IsRendezvous {
		rendezvousPeerCount := 0
		start := time.Now()
		peerChan, err := m.pubRoutingDiscovery.FindPeers(m.ctx, TvbaseRendezvous)
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
			m.IsRendezvous = true
			tvLog.Logger.Debugf("tvBase->DiscoverRendezvousPeers: It took %v seconds succcess connect to the rendezvous peer:%v",
				time.Since(start).Seconds(), peer.ID.Pretty())

			go m.registPeerInfo(peer.ID)
		}

		if rendezvousPeerCount == 0 {
			tvLog.Logger.Infof("tvBase->DiscoverRendezvousPeers: The number of peers is equal to 0, wait 10 second to search again")
			time.Sleep(10 * time.Second)
		} else {
			tvLog.Logger.Infof("tvBase->DiscoverRendezvousPeers: The number of rendezvous peer is %v", rendezvousPeerCount)
			for _, cb := range m.rendezvousCbList {
				cb()
			}
		}
	}
}
