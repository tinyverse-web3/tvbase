package tvbase

import (
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/libp2p/go-libp2p/core/peer"
	drouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	tvbaseLog "github.com/tinyverse-web3/tvbase/common/log"
	tvbaseUtil "github.com/tinyverse-web3/tvbase/common/util"
)

const TvbaseRendezvous = "tvbase/discover-rendzvous/common"

func (m *TvBase) initRendezvous() error {
	if m.pubRoutingDiscovery == nil {
		handleNoNet := func(peerID peer.ID) error {
			if !m.IsExistConnectedPeer() {
				for _, c := range m.rendezvousChanList {
					select {
					case c <- false:
						tvbaseLog.Logger.Debugf("TvBase-initRendezvous: succ send isRendezvousChan")
					default:
						tvbaseLog.Logger.Debugf("TvBase-initRendezvous: no receiver for isRendezvousChan")
					}
				}
				m.isRendezvous = false
			}
			return nil
		}
		m.RegistNotConnectedCallback(handleNoNet)

		m.isRendezvous = false
		m.isDiscoverRendzvousing = false
		m.rendezvousChanList = make([]chan bool, 0)
		m.pubRoutingDiscovery = drouting.NewRoutingDiscovery(m.dht)

		bo := backoff.NewExponentialBackOff()
		bo.InitialInterval = 2 * time.Second
		bo.Multiplier = 2
		bo.MaxInterval = 10 * time.Second
		bo.MaxElapsedTime = 0 // never stop

		go func() {
			t := backoff.NewTicker(bo)
			defer t.Stop()
			for {
				select {
				case <-t.C:
					if m.IsExistConnectedPeer() {
						tvbaseUtil.PubsubAdvertise(m.ctx, m.pubRoutingDiscovery, TvbaseRendezvous)
						return
					}
				case <-m.ctx.Done():
					return
				}
			}
		}()
	}
	return nil
}

func (m *TvBase) DiscoverRendezvousPeers() {
	tvbaseLog.Logger.Debugf("tvBase->DiscoverRendezvousPeers begin")
	bo := backoff.NewExponentialBackOff()
	bo.InitialInterval = 1 * time.Second
	bo.Multiplier = 2
	bo.MaxInterval = 10 * time.Second
	bo.MaxElapsedTime = 0 // never stop
	t := backoff.NewTicker(bo)
	defer t.Stop()

	for !m.isRendezvous {
		m.isDiscoverRendzvousing = true
		rendezvousPeerCount := 0
		start := time.Now()
		peerAddrInfoChan, err := m.pubRoutingDiscovery.FindPeers(m.ctx, TvbaseRendezvous)
		if err != nil {
			tvbaseLog.Logger.Errorf("tvBase->DiscoverRendezvousPeers: Searching rendezvous peer error: %v", err)
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
			tvbaseLog.Logger.Debugf("tvBase->DiscoverRendezvousPeers:\npeerAddrInfo.Addrs: %+v", peerAddrInfo.Addrs)
			wg.Add(1)
			go func(addrInfo peer.AddrInfo) {
				defer wg.Done()
				err := m.host.Connect(m.ctx, addrInfo)
				if err != nil {
					tvbaseLog.Logger.Warnf("tvBase->DiscoverRendezvousPeers:\nFail connect to the rendezvous addrInfo: %+v, error: %+v", addrInfo, err)
					return
				}
				rendezvousPeerCount++
				tvbaseLog.Logger.Debugf("tvBase->DiscoverRendezvousPeers:\nIt took %+v seconds succcess connect to the rendezvous peerID: %+v",
					time.Since(start).Seconds(), addrInfo.ID)
				m.registPeerInfo(addrInfo.ID)
			}(peerAddrInfo)
		}
		wg.Wait()
		if rendezvousPeerCount == 0 {
			tvbaseLog.Logger.Debugf("tvBase->DiscoverRendezvousPeers:\nThe number of peers is equal to 0")
			select {
			case <-t.C:
				tvbaseLog.Logger.Debugf("tvBase->DiscoverRendezvousPeers: wait...")
			case <-m.ctx.Done():
				return
			}
		} else {
			tvbaseLog.Logger.Debugf("tvBase->DiscoverRendezvousPeers:\nThe number of rendezvous peer is %+v", rendezvousPeerCount)
			for _, c := range m.rendezvousChanList {
				select {
				case c <- true:
					tvbaseLog.Logger.Debugf("TvBase-DiscoverRendezvousPeers: succ send isRendezvousChan")
				default:
					tvbaseLog.Logger.Debugf("TvBase-DiscoverRendezvousPeers: no receiver for isRendezvousChan")
				}
			}
			m.isRendezvous = true
			tvbaseLog.Logger.Infof("tvBase->DiscoverRendezvousPeers: succ rendezvous")
			m.PrintDiagnosisInfo()
		}
		m.isDiscoverRendzvousing = false
	}
	tvbaseLog.Logger.Debugf("tvBase->DiscoverRendezvousPeers end")
}

func (m *TvBase) RegistRendezvousChan() chan bool {
	c := make(chan bool)
	m.rendezvousChanList = append(m.rendezvousChanList, c)
	return c
}

func (m *TvBase) UnregistRendezvousChan(rc chan bool) {
	for i, c := range m.rendezvousChanList {
		if c == rc {
			close(c)
			m.rendezvousChanList = append(m.rendezvousChanList[:i], m.rendezvousChanList[i+1:]...)
			return
		}
	}
}

func (m *TvBase) GetIsRendezvous() bool {
	return m.isRendezvous
}
