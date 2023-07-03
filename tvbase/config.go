package tvbase

import (
	"context"
	"os"
	"strings"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/metrics"
	"github.com/libp2p/go-libp2p/core/pnet"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	libp2ptls "github.com/libp2p/go-libp2p/p2p/security/tls"
	quic "github.com/libp2p/go-libp2p/p2p/transport/quic"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	"github.com/libp2p/go-libp2p/p2p/transport/websocket"
	webtransport "github.com/libp2p/go-libp2p/p2p/transport/webtransport"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	"github.com/tinyverse-web3/tvbase/common"
	"github.com/tinyverse-web3/tvbase/common/config"
	"github.com/tinyverse-web3/tvbase/common/db"
	"github.com/tinyverse-web3/tvbase/common/identity"
	tvLog "github.com/tinyverse-web3/tvbase/common/log"
	"github.com/tinyverse-web3/tvbase/dht"
	mamask "github.com/whyrusleeping/multiaddr-filter"
)

func (m *Tvbase) initConfig(rootPath string) error {
	cfg := config.NewDefaultNodeConfig()
	err := config.InitConfig(rootPath, &cfg)
	if err != nil {
		tvLog.Logger.Errorf("infrasture->initConfig: error: %v", err)
		return err
	}
	m.nodeCfg = &cfg
	return nil
}

func (m *Tvbase) initKey(rootPath string) (crypto.PrivKey, pnet.PSK, error) {
	var err error
	identityPath := rootPath + identity.IdentityFileName
	_, err = os.Stat(identityPath)
	if os.IsNotExist(err) {
		identity.GenIdenityFile2Print(rootPath)
	}
	privteKey, err := identity.LoadIdentity(identityPath)
	if err != nil {
		return nil, nil, err
	}

	swarmPsk, fprint, err := identity.LoadSwarmKey(rootPath + identity.SwarmPskFileName)
	if err != nil {
		tvLog.Logger.Infof("no private swarm key")
	}
	if swarmPsk != nil {
		tvLog.Logger.Infof("PSK detected, private identity: %x\n", fprint)
	}
	return privteKey, swarmPsk, nil
}

func (m *Tvbase) createOpts(ctx context.Context, privateKey crypto.PrivKey, swamPsk pnet.PSK) ([]libp2p.Option, error) {
	var err error
	var opts []libp2p.Option
	opts, err = m.createCommonOpts(privateKey, swamPsk)
	if err != nil {
		tvLog.Logger.Errorf("infrasture->createOpts->createCommonOpts: error: %v", err)
		return nil, err
	}

	m.dhtDatastore, err = db.CreateDataStore(m.nodeCfg.DHT.DatastorePath, m.nodeCfg.Mode)
	if err != nil {
		tvLog.Logger.Errorf("infrasture->createOpts->createDataStore: error: %v", err)
		return nil, err
	}

	dkvsOpts, err := dht.CreateOpts(ctx, &(m.dht), m.dhtDatastore, m.nodeCfg.Mode, m.nodeCfg.DHT.ProtocolPrefix)
	if err != nil {
		tvLog.Logger.Errorf("infrasture->createOpts->dth.createOpts: error: %v", err)
		return nil, err
	}
	opts = append(opts, dkvsOpts...)
	return opts, nil
}

func (m *Tvbase) createNATOpts() ([]libp2p.Option, error) {
	var opts []libp2p.Option

	// Let this host use the DHT to find other hosts
	// If you want to help other peers to figure out if they are behind
	// NATs, you can launch the server-side of AutoNAT too (AutoRelay already runs the client)
	// This service is highly rate-limited and should not cause any performance issues.
	// for service node

	switch m.nodeCfg.AutoNAT.ServiceMode {
	default:
		panic("BUG: unhandled autonat service mode")
	case config.AutoNATServiceDisabled:
	case config.AutoNATServiceUnset:
		// TODO
		//
		// We're enabling the AutoNAT service by default on _all_ nodes
		// for the moment.
		//
		// We should consider disabling it by default if the dht is set
		// to dhtclient.
		fallthrough
	case config.AutoNATServiceEnabled:
		if !m.nodeCfg.Swarm.DisableNatPortMap {
			opts = append(opts, libp2p.EnableNATService())
			if m.nodeCfg.AutoNAT.Throttle != nil { // todo need to config
				opts = append(opts,
					libp2p.AutoNATServiceRateLimit(
						m.nodeCfg.AutoNAT.Throttle.GlobalLimit,
						m.nodeCfg.AutoNAT.Throttle.PeerLimit,
						m.nodeCfg.AutoNAT.Throttle.Interval,
					),
				)
			}
		}
	}

	switch m.nodeCfg.Mode {
	case config.LightMode:
		opts = append(opts,
			// for client node, use default host NATManager,
			// attempt to open a port in your network's firewall using UPnP
			libp2p.NATPortMap(),
		)
	case config.FullMode:
	}

	return opts, nil
}

func (m *Tvbase) createTransportOpts(isPrivateSwarm bool) ([]libp2p.Option, error) {
	var opts []libp2p.Option
	opts = append(opts,
		libp2p.Transport(tcp.NewTCPTransport, tcp.WithMetrics()),
		libp2p.Transport(websocket.New),
	)

	// it is not support transport for private network
	if !(isPrivateSwarm) {
		opts = append(opts, libp2p.Transport(quic.NewTransport), libp2p.Transport(webtransport.New))
	}
	return opts, nil
}

func (m *Tvbase) createCommonOpts(privateKey crypto.PrivKey, swarmPsk pnet.PSK) ([]libp2p.Option, error) {
	var opts []libp2p.Option

	opts = append(opts,
		libp2p.UserAgent(common.GetUserAgentVersion()),
		libp2p.Identity(privateKey),
		libp2p.ListenAddrStrings(m.nodeCfg.Network.ListenAddrs...),
		libp2p.Security(libp2ptls.ID, libp2ptls.New),
		libp2p.Security(noise.ID, noise.New),
	)

	// smux
	res, err := makeSmuxTransportOption(m.nodeCfg.Swarm.Transports)
	if err != nil {
		tvLog.Logger.Errorf("infrasture->createCommonOpts: error: %v", err)
		return nil, err
	}
	opts = append(opts, res)

	// Libp2pForceReachability -- for debug
	switch m.nodeCfg.Network.Libp2pForceReachability {
	case "public":
		opts = append(opts, libp2p.ForceReachabilityPublic())
	case "private":
		opts = append(opts, libp2p.ForceReachabilityPrivate())
	}

	// private swarm network
	if swarmPsk != nil {
		opts = append(opts, libp2p.PrivateNetwork(swarmPsk))
	}

	// transport
	transportOpts, err := m.createTransportOpts(swarmPsk != nil)
	if err != nil {
		return nil, err
	}
	opts = append(opts, transportOpts...)

	// annouceAddrs
	if len(m.nodeCfg.Network.AnnounceAddrs) > 0 {
		var announce []ma.Multiaddr
		for _, s := range m.nodeCfg.Network.AnnounceAddrs {
			a := ma.StringCast(s)
			announce = append(announce, a)
		}
		opts = append(opts,
			libp2p.AddrsFactory(func([]ma.Multiaddr) []ma.Multiaddr {
				return announce
			}),
		)
	} else {
		if m.nodeCfg.Mode == config.FullMode && !m.nodeCfg.Network.IsLocalNet {
			opts = append(opts,
				libp2p.AddrsFactory(func(addrs []ma.Multiaddr) []ma.Multiaddr {
					announce := make([]ma.Multiaddr, 0, len(addrs))
					for _, a := range addrs {
						if manet.IsPublicAddr(a) {
							announce = append(announce, a)
						}
					}
					return announce
				}),
			)
		} else {
			opts = append(opts,
				libp2p.AddrsFactory(func(addrs []ma.Multiaddr) []ma.Multiaddr {
					announce := make([]ma.Multiaddr, 0, len(addrs))
					for _, a := range addrs {
						addrInfo := strings.Split(a.String(), "/")
						if len(addrInfo) < 3 {
							continue
						}
						ip := addrInfo[2]
						if ip != "127.0.0.1" {
							announce = append(announce, a)
						}
					}
					return announce
				}),
			)
		}
	}

	// connection manager
	cm, err := connmgr.NewConnManager(
		m.nodeCfg.ConnMgr.ConnMgrLo,
		m.nodeCfg.ConnMgr.ConnMgrHi,
		connmgr.WithGracePeriod(m.nodeCfg.ConnMgr.ConnMgrGrace),
	)
	if err != nil {
		tvLog.Logger.Errorf("infrasture->createCommonOpts: error: %v", err)
		return nil, err
	}
	opts = append(opts,
		libp2p.ConnectionManager(cm),
	)

	// nat
	natOpts, err := m.createNATOpts()
	if err != nil {
		tvLog.Logger.Errorf("infrasture->createCommonOpts: error: %v", err)
		return nil, err
	}
	opts = append(opts, natOpts...)

	//relay
	relayOpts, err := m.createRelayOpts()
	if err != nil {
		tvLog.Logger.Errorf("infrasture->createCommonOpts: error: %v", err)
		return nil, err
	}
	opts = append(opts, relayOpts...)

	switch m.nodeCfg.Mode {
	case config.LightMode:
		// holePunching
		opts = append(opts,
			libp2p.EnableHolePunching(),
		)

		// metric
		opts = append(opts,
			libp2p.DisableMetrics(),
		)
	case config.FullMode:
		// resource manager
		rmgr, err := m.initResourceManager()
		if err != nil {
			tvLog.Logger.Errorf("infrasture->createCommonOpts: error: %v", err)
			return nil, err
		}
		opts = append(opts, libp2p.ResourceManager(rmgr))
		m.resourceManager = rmgr

		// BandwidthCounter
		if !m.nodeCfg.Swarm.DisableBandwidthMetrics {
			reporter := metrics.NewBandwidthCounter()
			opts = append(opts, libp2p.BandwidthReporter(reporter))
		}

		// connection gater filters
		if len(m.nodeCfg.Swarm.AddrFilters) > 0 {
			filter := ma.NewFilters()
			opts = append(opts, libp2p.ConnectionGater((*filtersConnectionGater)(filter)))
			for _, addr := range m.nodeCfg.Swarm.AddrFilters {
				f, err := mamask.NewMask(addr)
				if err != nil {
					tvLog.Logger.Errorf("Infrasture->createCommonOpts: incorrectly formatted address filter in config: %s", addr)
					return nil, err
				}
				filter.AddFilter(*f, ma.ActionDeny)
			}
		}
	}

	return opts, nil
}
