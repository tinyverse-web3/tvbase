package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	ipfsLog "github.com/ipfs/go-log/v2"
	"github.com/ipfs/kubo/config"
	"github.com/libp2p/go-libp2p/core/peer"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	relayv2 "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
)

type NodeMode int32

const (
	FullMode NodeMode = iota
	LightMode
)

const (
	NodeConfigFileName = "config.json"
	DefaultPort        = "9000"
)

// NodeConfig stores the full configuration of the relays, ACLs and other settings
// that influence behaviour of a relay daemon.
type NodeConfig struct {
	RootPath     string
	Mode         NodeMode
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
	Log          LogConfig
	Disc         DiscConfig
}

type DiscConfig struct {
	EnableProfile bool
}

type LogConfig struct {
	AllLogLevel  ipfsLog.LogLevel
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
	MaxMailboxPubsubCount int
	KeepMailboxMsgDay     int
	DatastorePath         string
}

type MetricsConfig struct {
	ApiPort int
}

// var DefaultNodeCfg NodeConfig = NewDefaultNodeConfig()

// returns a default relay configuration using default resource
func NewDefaultNodeConfig() NodeConfig {
	return NodeConfig{
		Mode: LightMode,
		Network: NetworkConfig{
			IsLocalNet: false,
			ListenAddrs: []string{
				"/ip4/0.0.0.0/udp/" + DefaultPort + "/quic",
				"/ip6/::/udp/" + DefaultPort + "/quic",
				"/ip4/0.0.0.0/tcp/" + DefaultPort,
				"/ip6/::/tcp/" + DefaultPort,
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
			MaxMailboxPubsubCount: 1000,
			KeepMailboxMsgDay:     30,
			DatastorePath:         "msg_data",
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
			AllLogLevel: ipfsLog.LevelError,
			ModuleLevels: map[string]string{
				"autorelay":      "debug",
				"tvbase":         "debug",
				"dkvs":           "debug",
				"dmsg":           "debug",
				"customProtocol": "debug",
			},
		},
		Disc: DiscConfig{
			EnableProfile: false,
		},
		Metrics: MetricsConfig{
			ApiPort: 2112,
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

func GenConfigFile(rootPath string, nodeCfg *NodeConfig) error {
	rootPath = strings.Trim(rootPath, " ")
	if rootPath == "" {
		rootPath = "."
	}
	if !strings.HasSuffix(rootPath, string(filepath.Separator)) {
		rootPath = rootPath + string(filepath.Separator)
	}
	nodeCfgPath := rootPath + NodeConfigFileName
	_, err := os.Stat(nodeCfgPath)
	if !os.IsNotExist(err) {
		err = nodeCfg.LoadConfig(nodeCfgPath)
		if err != nil {
			return err
		}
		defaultCfg := NewDefaultNodeConfig()
		err := mergeJSON(nodeCfg, &defaultCfg)
		if err != nil {
			return err
		}
	}
	file, err := json.MarshalIndent(nodeCfg, "", " ")
	if err != nil {
		return err
	}
	err = os.WriteFile(nodeCfgPath, file, 0644)
	if err != nil {
		return err
	}
	return nil
}

func InitConfig(rootPath string, nodeCfg *NodeConfig) error {
	rootPath = strings.Trim(rootPath, " ")
	if rootPath == "" {
		rootPath = "."
	}
	if !strings.HasSuffix(rootPath, string(filepath.Separator)) {
		rootPath = rootPath + string(filepath.Separator)
	}
	nodeCfgPath := rootPath + NodeConfigFileName
	_, err := os.Stat(nodeCfgPath)
	if os.IsNotExist(err) {
		file, _ := json.MarshalIndent(nodeCfg, "", " ")
		if err := os.WriteFile(rootPath+NodeConfigFileName, file, 0644); err != nil {
			return err
		}
	}
	err = nodeCfg.LoadConfig(nodeCfgPath)
	if err != nil {
		return err
	}
	if !filepath.IsAbs(nodeCfg.DHT.DatastorePath) {
		nodeCfg.DHT.DatastorePath = rootPath + nodeCfg.DHT.DatastorePath
	}
	if !filepath.IsAbs(nodeCfg.DMsg.DatastorePath) {
		nodeCfg.DMsg.DatastorePath = rootPath + nodeCfg.DMsg.DatastorePath
	}
	return nil
}

func mergeJSON(srcObj, destObj any) error {
	srcBytes, _ := json.Marshal(srcObj)
	destBytes, _ := json.Marshal(destObj)

	var srcMap map[string]any
	var destMap map[string]any
	err := json.Unmarshal(srcBytes, &srcMap)
	if err != nil {
		return err
	}
	err = json.Unmarshal(destBytes, &destMap)
	if err != nil {
		return err
	}

	mergeJsonFields(srcMap, destMap)

	result, err := json.Marshal(srcMap)
	if err != nil {
		return err
	}
	err = json.Unmarshal(result, &srcObj)
	if err != nil {
		return err
	}
	return nil
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
