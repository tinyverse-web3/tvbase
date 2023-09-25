package config

import (
	"time"

	"github.com/ipfs/kubo/config"
	"github.com/libp2p/go-libp2p/core/peer"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	relayv2 "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
	"github.com/tinyverse-web3/tvbase/common/define"
)

const (
	LightPort   = "0"
	ServicePort = "9000"
)

// TvbaseConfig stores the full configuration of the relays, ACLs and other settings
// that influence behaviour of a relay daemon.
type TvbaseConfig struct {
	Mode         define.NodeMode
	Network      NetworkConfig
	Swarm        config.SwarmConfig
	AutoNAT      AutoNATConfig
	ConnMgr      ConnMgrConfig
	Relay        RelayConfig
	ACL          ACLConfig
	Bootstrap    BootstrapConfig
	PartialLimit rcmgr.PartialLimitConfig
	DHT          DHTConfig
	DMsg         DMsgConfig
	Metrics      MetricsConfig
	CoreHttp     CoreHttpConfig
	Disc         DiscConfig
}

type DiscConfig struct {
	EnableProfile bool
}

// NetworkConfig controls listen and annouce settings for the libp2p host.
type NetworkConfig struct {
	IsLocalNet              bool
	ListenAddrs             []string
	AnnounceAddrs           []string
	Libp2pForceReachability string
	Peers                   []peer.AddrInfo
	EnableMdns              bool
}

// AutoNATConfig controls the libp2p auto NAT settings.
type AutoNATServiceMode int

const (
	// AutoNATServiceUnset indicates that the user has not set the
	// AutoNATService mode.
	//
	// When unset, nodes configured to be public DHT nodes will _also_
	// perform limited AutoNAT dialbacks.
	AutoNATServiceUnset AutoNATServiceMode = iota
	// AutoNATServiceEnabled indicates that the user has enabled the
	// AutoNATService.
	AutoNATServiceEnabled
	// AutoNATServiceDisabled indicates that the user has disabled the
	// AutoNATService.
	AutoNATServiceDisabled
)

// AutoNATConfig configures the node's AutoNAT subsystem.
type AutoNATConfig struct {
	// ServiceMode configures the node's AutoNAT service mode.
	ServiceMode AutoNATServiceMode `json:",omitempty"`

	// Throttle configures AutoNAT dialback throttling.
	//
	// If unset, the conservative libp2p defaults will be unset. To help the
	// network, please consider setting this and increasing the limits.
	//
	// By default, the limits will be a total of 30 dialbacks, with a
	// per-peer max of 3 peer, resetting every minute.
	Throttle *AutoNATThrottleConfig `json:",omitempty"`
}

type AutoNATThrottleConfig struct {
	// GlobalLimit and PeerLimit sets the global and per-peer dialback
	// limits. The AutoNAT service will only perform the specified number of
	// dialbacks per interval.
	//
	// Setting either to 0 will disable the appropriate limit.
	GlobalLimit, PeerLimit int

	// Interval specifies how frequently this node should reset the
	// global/peer dialback limits.
	//
	// When unset, this defaults to 1 minute.
	Interval time.Duration `json:",omitempty"`
}

// ConnMgrConfig controls the libp2p connection manager settings.
type ConnMgrConfig struct {
	ConnMgrLo    int
	ConnMgrHi    int
	ConnMgrGrace time.Duration
}

// RelayConfig controls activation of V2 circuits and resouce configuration
// for them.
type RelayConfig struct {
	Enabled   bool
	Resources relayv2.Resources
}

// ACLConfig provides filtering configuration to allow specific peers or
// subnets to be fronted by relays. In V2, this specifies the peers/subnets
// that are able to make reservations on the relay. In V1, this specifies the
// peers/subnets that can be contacted through the relays.
type ACLConfig struct {
	AllowPeers   []string
	AllowSubnets []string
}

type BootstrapConfig struct {
	BootstrapPeers []string
}

type DHTConfig struct {
	DatastorePath  string
	ProtocolPrefix string
	ProtocolID     string
	MaxRecordAge   time.Duration
}

type DMsgConfig struct {
	MaxMsgCount     int
	KeepMsgDay      int
	MaxMailboxCount int
	KeepMailboxDay  int
	MaxChannelCount int
	KeepChannelDay  int
	DatastorePath   string
}

type MetricsConfig struct {
	ApiPort int
}

type CoreHttpConfig struct {
	ApiPort int
}

func NewDefaultTvbaseConfig() *TvbaseConfig {
	ret := TvbaseConfig{
		Mode: define.LightMode,
		Network: NetworkConfig{
			IsLocalNet:              false,
			ListenAddrs:             []string{},
			Libp2pForceReachability: "",
			Peers:                   []peer.AddrInfo{},
			EnableMdns:              true,
		},
		Swarm: config.SwarmConfig{
			AddrFilters:             nil,
			ConnMgr:                 config.ConnMgr{},
			DisableBandwidthMetrics: false,
			DisableNatPortMap:       false,
			RelayClient:             config.RelayClient{},
			RelayService:            config.RelayService{},
			ResourceMgr:             config.ResourceMgr{},
			Transports:              config.Transports{},
		},
		AutoNAT: AutoNATConfig{
			ServiceMode: AutoNATServiceEnabled,
			Throttle:    nil,
		},
		ConnMgr: ConnMgrConfig{
			ConnMgrLo:    320,              // 512,
			ConnMgrHi:    960,              // 768,
			ConnMgrGrace: 20 * time.Second, // 2 * time.Minute,
		},
		Bootstrap: BootstrapConfig{
			BootstrapPeers: []string{
				// TODO add 4 dns peer bootstrap
				// "/dnsaddr/bootstrap.tinyverse.space/p2p/xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
				"/ip4/103.103.245.177/tcp/9000/p2p/12D3KooWFvycqvSRcrPPSEygV7pU6Vd2BrpGsMMFvzeKURbGtMva",
				// TODO add udp peer bootstrap
				// "/ip4/103.103.245.177/udp/9000/quic/p2p/12D3KooWFvycqvSRcrPPSEygV7pU6Vd2BrpGsMMFvzeKURbGtMva",
				"/ip4/156.251.179.141/tcp/9000/p2p/12D3KooWH743TTDbp2RLsLL2t2vVNdtKpm3AMyZffRVx5psBbbZ3",
				// TODO add udp peer bootstrap
				// "/ip4/156.251.179.141/udp/9000/quic/p2p/12D3KooWH743TTDbp2RLsLL2t2vVNdtKpm3AMyZffRVx5psBbbZ3",
				"/ip4/39.108.104.19/tcp/9000/p2p/12D3KooWNfbV19fQ9d39K84fUeFRmc6i4koEVNio9L6fPFtyPC9V",
				// TODO add udp peer bootstrap
				// "/ip4/39.108.104.19/udp/9000/quic/p2p/12D3KooWH743TTDbp2RLsLL2t2vVNdtKpm3AMyZffRVx5psBbbZ3",
			},
		},
		PartialLimit: rcmgr.PartialLimitConfig{},
		ACL: ACLConfig{
			AllowPeers:   []string{},
			AllowSubnets: []string{},
		},
		DHT: DHTConfig{
			DatastorePath:  "dht_data",
			ProtocolPrefix: "/tvnode",
			ProtocolID:     "/kad/1.0.0",
			MaxRecordAge:   time.Hour * 24 * 365 * 100,
		},
		DMsg: DMsgConfig{
			MaxMsgCount:     1000,
			KeepMsgDay:      30,
			MaxMailboxCount: 1000,
			KeepMailboxDay:  30,
			MaxChannelCount: 1000,
			KeepChannelDay:  30,
			DatastorePath:   "msg_data",
		},
		Relay: RelayConfig{
			Enabled: true,
			// default relayv2.DefaultResources()
			Resources: relayv2.Resources{
				// default relayv2.DefaultLimit()
				Limit: &relayv2.RelayLimit{
					Duration: 2 * time.Minute,
					Data:     1 << 17, // 128K
				},
				ReservationTTL: time.Hour,
				// Use the default to verify whether it is a more appropriate value, can refer to the default value used in kubo
				MaxReservations:        1280, // default 128
				MaxCircuits:            32,   // default 16
				BufferSize:             2048,
				MaxReservationsPerPeer: 100, //default 4
				MaxReservationsPerIP:   200, //default 8
				MaxReservationsPerASN:  32,  //default 32
			},
		},
		Disc: DiscConfig{
			EnableProfile: false,
		},
		Metrics: MetricsConfig{
			ApiPort: 2112,
		},
		CoreHttp: CoreHttpConfig{
			ApiPort: 9099,
		},
	}
	return &ret
}

func (cfg *TvbaseConfig) InitMode(mode define.NodeMode) {
	cfg.Mode = mode
	switch cfg.Mode {
	case define.ServiceMode:
		cfg.Network.ListenAddrs = []string{
			"/ip4/0.0.0.0/udp/" + ServicePort + "/quic",
			"/ip6/::/udp/" + ServicePort + "/quic",
			"/ip4/0.0.0.0/tcp/" + ServicePort,
			"/ip6/::/tcp/" + ServicePort,
		}
	case define.LightMode:
		cfg.Network.ListenAddrs = []string{
			"/ip4/0.0.0.0/udp/" + LightPort + "/quic",
			"/ip6/::/udp/" + LightPort + "/quic",
			"/ip4/0.0.0.0/tcp/" + LightPort,
			"/ip6/::/tcp/" + LightPort,
		}
	}
}

func (cfg *TvbaseConfig) ClearBootstrapPeers() {
	cfg.Bootstrap.BootstrapPeers = []string{}
}

func (cfg *TvbaseConfig) AddBootstrapPeer(peer string) {
	cfg.Bootstrap.BootstrapPeers = append(cfg.Bootstrap.BootstrapPeers, peer)
}

func (cfg *TvbaseConfig) SetLocalNet(isLocalNet bool) {
	cfg.Network.IsLocalNet = isLocalNet
}

func (cfg *TvbaseConfig) SetMdns(enable bool) {
	cfg.Network.EnableMdns = enable
}

func (cfg *TvbaseConfig) SetDhtProtocolPrefix(prefix string) {
	cfg.DHT.ProtocolPrefix = prefix
}