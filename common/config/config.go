package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"time"

	"github.com/ipfs/kubo/config"
	"github.com/libp2p/go-libp2p/core/peer"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	relayv2 "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
	"github.com/tinyverse-web3/tvbase/common/define"
)

var (
	NodeConfigFileName = "config.json"
	LightPort          = "0"
	ServicePort        = "9000"
)

// NodeConfig stores the full configuration of the relays, ACLs and other settings
// that influence behaviour of a relay daemon.
type NodeConfig struct {
	RootPath     string
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
	Log          LogConfig
	Disc         DiscConfig
}

type DiscConfig struct {
	EnableProfile bool
}

type LogConfig struct {
	ModuleLevels map[string]string
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
}

type DMsgConfig struct {
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

// returns a default relay configuration using default resource
func NewDefaultNodeConfig(mode define.NodeMode) *NodeConfig {
	listenPort := LightPort
	switch mode {
	case define.ServiceMode:
		listenPort = ServicePort
	}
	return &NodeConfig{
		Mode: define.LightMode,
		Network: NetworkConfig{
			IsLocalNet: false,
			ListenAddrs: []string{
				"/ip4/0.0.0.0/udp/" + listenPort + "/quic",
				"/ip6/::/udp/" + listenPort + "/quic",
				"/ip4/0.0.0.0/tcp/" + listenPort,
				"/ip6/::/tcp/" + listenPort,
			},
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
		},
		DMsg: DMsgConfig{
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
		Log: LogConfig{
			ModuleLevels: map[string]string{
				"tvbase":      "debug",
				"dkvs":        "debug",
				"dmsg":        "debug",
				"tvnode":      "debug",
				"tvnodelight": "debug",
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
}

// LoadConfig reads a relay daemon JSON configuration from the given path.
// The configuration is first initialized with DefaultConfig, so all unset
// fields will take defaults from there.
func (cfg *NodeConfig) LoadConfig(filePath string) error {
	if filePath != "" {
		cfgFile, err := os.Open(filePath)
		if err != nil {
			return err
		}
		defer cfgFile.Close()

		decoder := json.NewDecoder(cfgFile)
		err = decoder.Decode(&cfg)
		if err != nil {
			return err
		}
	}
	return nil
}

func Merge2GenConfigFile(rootPath string, mode define.NodeMode) error {
	cfg := &NodeConfig{}
	cfgPath := rootPath + NodeConfigFileName
	defaultCfg := NewDefaultNodeConfig(mode)
	_, err := os.Stat(cfgPath)
	if os.IsNotExist(err) {
		cfg = defaultCfg
	} else {
		err = cfg.LoadConfig(cfgPath)
		if err != nil {
			return err
		}
		cfg, err = mergeJSON(cfg, defaultCfg)
		if err != nil {
			return err
		}
	}
	file, err := json.MarshalIndent(cfg, "", " ")
	if err != nil {
		return err
	}
	err = os.WriteFile(cfgPath, file, 0644)
	if err != nil {
		return err
	}
	return nil
}

func InitNodeConfigFile(rootPath string, defaultMode define.NodeMode) (*NodeConfig, error) {
	var cfg *NodeConfig = &NodeConfig{}
	_, err := os.Stat(rootPath)
	if os.IsNotExist(err) {
		err := os.MkdirAll(rootPath, 0755)
		if err != nil {
			fmt.Println("GenConfig: Failed to create directory:", err)
			return nil, err
		}
	}
	cfgPath := rootPath + NodeConfigFileName
	_, err = os.Stat(cfgPath)
	if os.IsNotExist(err) {
		cfg = NewDefaultNodeConfig(defaultMode)
		file, _ := json.MarshalIndent(cfg, "", " ")
		if err := os.WriteFile(rootPath+NodeConfigFileName, file, 0644); err != nil {
			fmt.Println("InitConfig: Failed to WriteFile:", err)
			return nil, err
		}
	} else {
		err = cfg.LoadConfig(cfgPath)
		if err != nil {
			return nil, err
		}
	}

	cfg.RootPath = rootPath
	if !filepath.IsAbs(cfg.DHT.DatastorePath) {
		cfg.DHT.DatastorePath = rootPath + cfg.DHT.DatastorePath
	}
	if !filepath.IsAbs(cfg.DMsg.DatastorePath) {
		cfg.DMsg.DatastorePath = rootPath + cfg.DMsg.DatastorePath
	}
	return cfg, nil
}

func mergeJSON(srcCfg, destCfg *NodeConfig) (*NodeConfig, error) {
	srcBytes, err := json.Marshal(srcCfg)
	if err != nil {
		return nil, err
	}
	destBytes, err := json.Marshal(destCfg)
	if err != nil {
		return nil, err
	}

	var srcMap map[string]any
	var destMap map[string]any
	err = json.Unmarshal(srcBytes, &srcMap)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(destBytes, &destMap)
	if err != nil {
		return nil, err
	}

	mergeJsonFields(srcMap, destMap)

	data, err := json.Marshal(srcMap)
	if err != nil {
		return nil, err
	}

	ret := &NodeConfig{}
	err = json.Unmarshal(data, ret)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func mergeJsonFields(srcObj, destObj map[string]any) error {
	for key, value := range destObj {
		if value == nil {
			continue
		}
		if reflect.TypeOf(value).Kind() == reflect.Map {
			if srcObj[key] == nil {
				srcObj[key] = make(map[string]any)
			}
			err := mergeJsonFields(srcObj[key].(map[string]any), value.(map[string]any))
			if err != nil {
				return err
			}
		} else {
			if srcObj[key] == nil {
				srcObj[key] = value
			}
		}
	}
	return nil
}
