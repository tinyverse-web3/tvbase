package tvbase

import (
	"sync"
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
	tvLog.Logger.Debugf("tvBase->DiscoverRendezvousPeers begin")
	for !m.IsRendezvous {
		rendezvousPeerCount := 0
		start := time.Now()
		peerAddrInfoChan, err := m.pubRoutingDiscovery.FindPeers(m.ctx, TvbaseRendezvous)
		if err != nil {
			tvLog.Logger.Errorf("tvBase->DiscoverRendezvousPeers: Searching rendezvous peer error: %v", err)
			continue
		}

		var wg sync.WaitGroup
		for peerAddrInfo := range peerAddrInfoChan {
			if peerAddrInfo.ID == m.host.ID() {
				continue
			}
			if len(peerAddrInfo.Addrs) == 0 {
				continue
			}
			tvLog.Logger.Debugf("tvBase->DiscoverRendezvousPeers: peerAddrInfo.Addrs:%v", peerAddrInfo.Addrs)
			wg.Add(1)
			go func(addrInfo peer.AddrInfo) {
				defer wg.Done()
				err := m.host.Connect(m.ctx, addrInfo)
				if err != nil {
					tvLog.Logger.Warnf("tvBase->DiscoverRendezvousPeers: Fail connect to the rendezvous addrInfo: %+v, error: %+v", addrInfo, err)
					return
				}
				rendezvousPeerCount++
				tvLog.Logger.Debugf("tvBase->DiscoverRendezvousPeers: It took %+v seconds succcess connect to the rendezvous peerID: %+v",
					time.Since(start).Seconds(), addrInfo.ID)
				m.registPeerInfo(addrInfo.ID)
			}(peerAddrInfo)
		}
		wg.Wait()
		if rendezvousPeerCount == 0 {
			tvLog.Logger.Debugf("tvBase->DiscoverRendezvousPeers: The number of peers is equal to 0, wait 10 second to search again")
			time.Sleep(10 * time.Second)
		} else {
			tvLog.Logger.Debugf("tvBase->DiscoverRendezvousPeers: The number of rendezvous peer is %v", rendezvousPeerCount)
			m.IsRendezvous = true
			for _, cb := range m.rendezvousCbList {
				cb()
			}
		}
	}
	tvLog.Logger.Debugf("tvBase->DiscoverRendezvousPeers end")
}
