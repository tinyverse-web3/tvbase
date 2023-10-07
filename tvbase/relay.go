package tvbase

import (
	"context"
	"fmt"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/host/autorelay"
	relayv2 "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
	"github.com/tinyverse-web3/tvbase/common/config"
	tvLog "github.com/tinyverse-web3/tvbase/common/log"
)

func (m *TvBase) createRelayOpts() ([]libp2p.Option, error) {
	enableRelayTransport := m.cfg.Swarm.Transports.Network.Relay.WithDefault(true)
	enableRelayService := m.cfg.Swarm.RelayService.Enabled.WithDefault(enableRelayTransport)
	enableRelayClient := m.cfg.Swarm.RelayClient.Enabled.WithDefault(enableRelayTransport)

	if !enableRelayTransport {
		if enableRelayService {
			tvLog.Logger.Error("tvBase->createRelayOpts: Failed to enable `Swarm.RelayService`, it requires `Swarm.Transports.Network.Relay` to be true.")

			return nil, fmt.Errorf("tvBase->createRelayOpts: failed to enable `Swarm.RelayService`, it requires `Swarm.Transports.Network.Relay` to be true")
		}
		if enableRelayClient {
			tvLog.Logger.Error("tvBase->createRelayOpts: Failed to enable `Swarm.RelayClient`, it requires `Swarm.Transports.Network.Relay` to be true.")
			return nil, fmt.Errorf("tvBase->createRelayOpts: failed to enable `Swarm.RelayClient`, it requires `Swarm.Transports.Network.Relay` to be true")
		}
	}

	var opts []libp2p.Option
	if enableRelayTransport {
		opts = append(opts, libp2p.EnableRelay())
	} else {
		opts = append(opts, libp2p.DisableRelay())
	}

	if enableRelayClient {
		// auto relay -- static relays
		if len(m.cfg.Swarm.RelayClient.StaticRelays) > 0 {
			staticRelays := make([]peer.AddrInfo, 0, len(m.cfg.Swarm.RelayClient.StaticRelays))
			for _, s := range m.cfg.Swarm.RelayClient.StaticRelays {
				var addr *peer.AddrInfo
				var err error
				addr, err = peer.AddrInfoFromString(s)
				if err != nil {
					return nil, err
				}
				staticRelays = append(staticRelays, *addr)
			}
			opts = append(opts, libp2p.EnableAutoRelayWithStaticRelays(staticRelays))
		}

		// auto relay -- dynamic find relays
		relayPeerSignal := make(chan peer.AddrInfo)
		opt := libp2p.EnableAutoRelayWithPeerSource(
			func(ctx context.Context, numPeers int) <-chan peer.AddrInfo {
				// TODO(9257): make this code smarter (have a state and actually try to grow the search outward) instead of a long running task just polling our K cluster.
				r := make(chan peer.AddrInfo)
				go func() {
					defer close(r)
					for ; numPeers != 0; numPeers-- {
						select {
						case v, ok := <-relayPeerSignal:
							if !ok {
								return
							}
							select {
							case r <- v:
							case <-ctx.Done():
								return
							}
						case <-ctx.Done():
							return
						}
					}
				}()
				return r
			},
			autorelay.WithMinInterval(0),
			autorelay.WithBootDelay(0*time.Second),
		)
		opts = append(opts, opt)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					tvLog.Logger.Debugf("tvBase->createRelayOpts: recovering from unexpected error in AutoRelayFeeder:", r)
				}
			}()

			// Feed peers more often right after the bootstrap, then backoff
			bo := backoff.NewExponentialBackOff()
			bo.InitialInterval = 15 * time.Second
			bo.Multiplier = 3
			bo.MaxInterval = 1 * time.Hour
			bo.MaxElapsedTime = 0 // never stop
			t := backoff.NewTicker(bo)
			defer t.Stop()
			for {
				select {
				case <-t.C:
				case <-m.ctx.Done():
					close(relayPeerSignal)
					return
				}

				// Always feed trusted IDs (Peering.Peers in the config)
				for _, trustedPeer := range m.cfg.Network.Peers {
					if len(trustedPeer.Addrs) == 0 {
						continue
					}
					select {
					case relayPeerSignal <- trustedPeer:
					case <-m.ctx.Done():
						close(relayPeerSignal)
						return
					}
				}

				if m.dht == nil || m.host == nil {
					/* noop due to missing dht.WAN. happens in some unit tests,
					   not worth fixing as we will refactor this after go-libp2p 0.20 */
					continue
				}

				closestPeers, err := m.dht.GetClosestPeers(m.ctx, m.host.ID().String())
				if err != nil {
					// no-op: usually 'failed to find any peer in table' during startup
					continue
				}
				for _, p := range closestPeers {
					addrs := m.host.Peerstore().Addrs(p)
					if len(addrs) == 0 {
						continue
					}
					dhtPeer := peer.AddrInfo{ID: p, Addrs: addrs}
					select {
					case <-t.C:
						continue
					case relayPeerSignal <- dhtPeer:
					case <-m.ctx.Done():
						close(relayPeerSignal)
						return
					}
				}
			}
		}()
	}
	switch m.cfg.Mode {
	case config.LightMode:

	case config.ServiceMode:
		// enable relay server
		var ropts []relayv2.Option
		if enableRelayService {
			def := m.cfg.Relay.Resources
			relayOpts := m.cfg.Swarm.RelayService
			ropts = append(ropts, relayv2.WithResources(relayv2.Resources{
				Limit: &relayv2.RelayLimit{
					Data:     def.Limit.Data,
					Duration: def.Limit.Duration,
				},
				MaxCircuits:            int(relayOpts.MaxCircuits.WithDefault(int64(def.MaxCircuits))),
				BufferSize:             int(relayOpts.BufferSize.WithDefault(int64(def.BufferSize))),
				ReservationTTL:         relayOpts.ReservationTTL.WithDefault(def.ReservationTTL),
				MaxReservations:        int(relayOpts.MaxReservations.WithDefault(int64(def.MaxReservations))),
				MaxReservationsPerIP:   int(relayOpts.MaxReservationsPerIP.WithDefault(int64(def.MaxReservationsPerIP))),
				MaxReservationsPerPeer: int(relayOpts.MaxReservationsPerPeer.WithDefault(int64(def.MaxReservationsPerPeer))),
				MaxReservationsPerASN:  int(relayOpts.MaxReservationsPerASN.WithDefault(int64(def.MaxReservationsPerASN))),
			}))
		}

		acl, err := NewACL(m.host, m.cfg.ACL)
		if err == nil {
			ropts = append(ropts, relayv2.WithACL(acl))
		}
		opts = append(opts, libp2p.EnableRelayService(ropts...))
	}

	return opts, nil
}
