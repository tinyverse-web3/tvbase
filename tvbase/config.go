package tvbase

import (
	"os"
	"strings"

	"github.com/libp2p/go-libp2p"
	kaddht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/metrics"
	"github.com/libp2p/go-libp2p/core/pnet"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/core/routing"
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
	tvConfig "github.com/tinyverse-web3/tvbase/common/config"
	"github.com/tinyverse-web3/tvbase/common/identity"
	tvLog "github.com/tinyverse-web3/tvbase/common/log"
	tvUtil "github.com/tinyverse-web3/tvbase/common/util"
	"github.com/tinyverse-web3/tvbase/dkvs"
	mamask "github.com/whyrusleeping/multiaddr-filter"
	"go.uber.org/fx"
)

func (m *TvBase) initConfig(rootPath string) error {
	cfg := tvConfig.NewDefaultNodeConfig()
	err := tvConfig.InitConfig(rootPath, &cfg)
	if err != nil {
		tvLog.Logger.Errorf("tvbase->initConfig: error: %v", err)
		return err
	}
	m.nodeCfg = &cfg
	return nil
}

func (m *TvBase) initKey(lc fx.Lifecycle) (crypto.PrivKey, pnet.PSK, error) {
	identityPath := m.nodeCfg.RootPath + identity.IdentityFileName
	_, err := os.Stat(identityPath)
	if os.IsNotExist(err) {
		identity.GenIdenityFile(m.nodeCfg.RootPath)
	}
	privteKey, err := identity.LoadIdentity(identityPath)
	if err != nil {
		return nil, nil, err
	}

	swarmPsk, fprint, err := identity.LoadSwarmKey(m.nodeCfg.RootPath + identity.SwarmPskFileName)
	if err != nil {
		tvLog.Logger.Infof("no private swarm key")
	}
	if swarmPsk != nil {
		tvLog.Logger.Infof("PSK detected, private identity: %x\n", fprint)
	}
	return privteKey, swarmPsk, nil
}

func (m *TvBase) createNATOpts() ([]libp2p.Option, error) {
	var opts []libp2p.Option

	// Let this host use the DHT to find other hosts
	// If you want to help other peers to figure out if they are behind
	// NATs, you can launch the server-side of AutoNAT too (AutoRelay already runs the client)
	// This service is highly rate-limited and should not cause any performance issues.
	// for service node

	switch m.nodeCfg.AutoNAT.ServiceMode {
	default:
		panic("BUG: unhandled autonat service mode")
	case tvConfig.AutoNATServiceDisabled:
	case tvConfig.AutoNATServiceUnset:
		// TODO
		//
		// We're enabling the AutoNAT service by default on _all_ nodes
		// for the moment.
		//
		// We should consider disabling it by default if the dht is set
		// to dhtclient.
		fallthrough
	case tvConfig.AutoNATServiceEnabled:
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
	case tvConfig.LightMode:
		opts = append(opts,
			// for client node, use default host NATManager,
			// attempt to open a port in your network's firewall using UPnP
			libp2p.NATPortMap(),
		)
	case tvConfig.FullMode:
	}

	return opts, nil
}

func (m *TvBase) createTransportOpts(isPrivateSwarm bool) ([]libp2p.Option, error) {
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

func (m *TvBase) createCommonOpts(privateKey crypto.PrivKey, swarmPsk pnet.PSK) ([]libp2p.Option, error) {
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
		tvLog.Logger.Errorf("tvbase->createCommonOpts: error: %v", err)
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
		if m.nodeCfg.Mode == tvConfig.FullMode && !m.nodeCfg.Network.IsLocalNet {
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
		tvLog.Logger.Errorf("tvbase->createCommonOpts: error: %v", err)
		return nil, err
	}
	opts = append(opts,
		libp2p.ConnectionManager(cm),
	)

	// nat
	natOpts, err := m.createNATOpts()
	if err != nil {
		tvLog.Logger.Errorf("tvbase->createCommonOpts: error: %v", err)
		return nil, err
	}
	opts = append(opts, natOpts...)

	//relay
	relayOpts, err := m.createRelayOpts()
	if err != nil {
		tvLog.Logger.Errorf("tvbase->createCommonOpts: error: %v", err)
		return nil, err
	}
	opts = append(opts, relayOpts...)

	switch m.nodeCfg.Mode {
	case tvConfig.LightMode:
		// holePunching
		opts = append(opts,
			libp2p.EnableHolePunching(),
		)

		// metric
		opts = append(opts,
			libp2p.DisableMetrics(),
		)
	case tvConfig.FullMode:
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

func (m *TvBase) createRouteOpt() (libp2p.Option, error) {
	var err error
	bsCfgPeers := m.GetConfig().Bootstrap.BootstrapPeers
	bspeers, err := tvUtil.ParseBootstrapPeers(bsCfgPeers)
	if err != nil {
		tvLog.Logger.Errorf("tvbase->tvUtil.ParseBootstrapPeers(bsCfgPeers): error: %v", err)
		return nil, err
	}
	opt := libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {
		var modeCfg kaddht.Option

		switch m.nodeCfg.Mode {
		case tvConfig.FullMode:
			modeCfg = kaddht.Mode(kaddht.ModeServer)
		case tvConfig.LightMode:
			modeCfg = kaddht.Mode(kaddht.ModeAuto)
		}
		m.dht, err = kaddht.New(m.ctx,
			h,
			kaddht.ProtocolPrefix(protocol.ID(m.nodeCfg.DHT.ProtocolPrefix)), // kaddht.ProtocolPrefix("/test"),
			kaddht.Validator(dkvs.Validator{}),                               // kaddht.NamespacedValidator("tinyverseNetwork", blankValidator{}),
			// EnableOptimisticProvide enables an optimization that skips the last hops of the provide process.
			// This works by using the network size estimator (which uses the keyspace density of queries)
			// to optimistically send ADD_PROVIDER requests when we most likely have found the last hop.
			// It will also run some ADD_PROVIDER requests asynchronously in the background after returning,
			// this allows to optimistically return earlier if some threshold number of RPCs have succeeded.
			// The number of background/in-flight queries can be configured with the OptimisticProvideJobsPoolSize
			// option.
			//
			// EXPERIMENTAL: This is an experimental option and might be removed in the future. Use at your own risk.
			kaddht.EnableOptimisticProvide(), // enable optimistic provide
			modeCfg,
			// BootstrapPeers configures the bootstrapping nodes that we will connect to to seed
			// and refresh our Routing Table if it becomes empty.
			kaddht.BootstrapPeers(bspeers...),
			kaddht.Datastore(m.dhtDatastore),
		)

		if err != nil {
			tvLog.Logger.Errorf("tvbase->createOpts: error: %v", err)
			return nil, err
		}

		return m.dht, nil
	})
	return opt, nil
}
