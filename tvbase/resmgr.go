package tvbase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/dustin/go-humanize"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core/node/libp2p/fd"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	rcmgrObs "github.com/libp2p/go-libp2p/p2p/host/resource-manager/obs"

	ma "github.com/multiformats/go-multiaddr"
	"github.com/pbnjay/memory"
	"github.com/prometheus/client_golang/prometheus"
	tvLog "github.com/tinyverse-web3/tvbase/common/log"
	"go.uber.org/zap"
)

// rcmgr.go
const NetLimitTraceFilename = "rcmgr.json.gz"

var ErrNoResourceMgr = fmt.Errorf("missing ResourceMgr: make sure the daemon is running with Swarm.ResourceMgr.Enabled")

func (m *TvBase) initResourceManager() (network.ResourceManager, error) {
	var manager network.ResourceManager

	tvLog.Logger.Debug("tvBase->initResourceManager: libp2p resource manager is enabled")
	limitConfig, msg, err := LimitConfig(m.cfg.Swarm, m.cfg.PartialLimit)
	if err != nil {
		tvLog.Logger.Errorf("tvBase->initResourceManager: creating final Resource Manager config: %w", err)
		return nil, err
	}

	if !isPartialConfigEmpty(m.cfg.PartialLimit) {
		tvLog.Logger.Warn(`tvBase->initResourceManager: libp2p-resource-limit-overrides.json has been loaded, "default" fields will be filled in with autocomputed defaults.`)
	}

	// We want to see this message on startup, that's why we are using fmt instead of log.
	tvLog.Logger.Debug(msg)

	if err := ensureConnMgrMakeSenseVsResourceMgr(limitConfig, m.cfg.Swarm); err != nil {
		return nil, err
	}

	str, err := rcmgrObs.NewStatsTraceReporter()
	if err != nil {
		tvLog.Logger.Errorf("tvBase->initResourceManager: rcmgrObs.NewStatsTraceReporter(): %v", err)
		return nil, err
	}

	ropts := []rcmgr.Option{rcmgr.WithMetrics(createRcmgrMetrics()), rcmgr.WithTraceReporter(str)}

	if len(m.cfg.Swarm.ResourceMgr.Allowlist) > 0 {
		var mas []ma.Multiaddr
		for _, maStr := range m.cfg.Swarm.ResourceMgr.Allowlist {
			ma, err := ma.NewMultiaddr(maStr)
			if err != nil {
				tvLog.Logger.Warnf("tvBase->initResourceManager: failed to parse multiaddr=%v for allowlist, skipping. err=%v", maStr, err)
				continue
			}
			mas = append(mas, ma)
		}
		ropts = append(ropts, rcmgr.WithAllowlistedMultiaddrs(mas))
		tvLog.Logger.Infof("tvBase->initResourceManager: Setting allowlist to: %v", mas)
	}

	if os.Getenv("LIBP2P_DEBUG_RCMGR") != "" {
		traceFilePath := filepath.Join(m.rootPath, NetLimitTraceFilename)
		ropts = append(ropts, rcmgr.WithTrace(traceFilePath))
	}

	limiter := rcmgr.NewFixedLimiter(limitConfig)

	manager, err = rcmgr.NewResourceManager(limiter, ropts...)
	if err != nil {
		return nil, err
	}
	lrm := &loggingResourceManager{
		clock:    clock.New(),
		logger:   &logging.Logger("resourcemanager").SugaredLogger,
		delegate: manager,
	}
	lrm.start(m.ctx)
	return lrm, nil
}

func isPartialConfigEmpty(cfg rcmgr.PartialLimitConfig) bool {
	var emptyResourceConfig rcmgr.ResourceLimits
	if cfg.System != emptyResourceConfig ||
		cfg.Transient != emptyResourceConfig ||
		cfg.AllowlistedSystem != emptyResourceConfig ||
		cfg.AllowlistedTransient != emptyResourceConfig ||
		cfg.ServiceDefault != emptyResourceConfig ||
		cfg.ServicePeerDefault != emptyResourceConfig ||
		cfg.ProtocolDefault != emptyResourceConfig ||
		cfg.ProtocolPeerDefault != emptyResourceConfig ||
		cfg.PeerDefault != emptyResourceConfig ||
		cfg.Conn != emptyResourceConfig ||
		cfg.Stream != emptyResourceConfig {
		return false
	}
	for _, v := range cfg.Service {
		if v != emptyResourceConfig {
			return false
		}
	}
	for _, v := range cfg.ServicePeer {
		if v != emptyResourceConfig {
			return false
		}
	}
	for _, v := range cfg.Protocol {
		if v != emptyResourceConfig {
			return false
		}
	}
	for _, v := range cfg.ProtocolPeer {
		if v != emptyResourceConfig {
			return false
		}
	}
	for _, v := range cfg.Peer {
		if v != emptyResourceConfig {
			return false
		}
	}
	return true
}

// LimitConfig returns the union of the Computed Default Limits and the User Supplied Override Limits.
func LimitConfig(cfg config.SwarmConfig, userResourceOverrides rcmgr.PartialLimitConfig) (limitConfig rcmgr.ConcreteLimitConfig, logMessageForStartup string, err error) {
	limitConfig, msg, err := createDefaultLimitConfig(cfg)
	if err != nil {
		return rcmgr.ConcreteLimitConfig{}, msg, err
	}

	// The logic for defaults and overriding with specified userResourceOverrides
	// is documented in docs/libp2p-resource-management.md.
	// Any changes here should be reflected there.

	// This effectively overrides the computed default LimitConfig with any non-"useDefault" values from the userResourceOverrides file.
	// Because of how how Build works, any rcmgr.Default value in userResourceOverrides
	// will be overriden with a computed default value.
	limitConfig = userResourceOverrides.Build(limitConfig)

	return limitConfig, msg, nil
}

type ResourceLimitsAndUsage struct {
	// This is duplicated from rcmgr.ResourceResourceLimits but adding *Usage fields.
	Memory               rcmgr.LimitVal64
	MemoryUsage          int64
	FD                   rcmgr.LimitVal
	FDUsage              int
	Conns                rcmgr.LimitVal
	ConnsUsage           int
	ConnsInbound         rcmgr.LimitVal
	ConnsInboundUsage    int
	ConnsOutbound        rcmgr.LimitVal
	ConnsOutboundUsage   int
	Streams              rcmgr.LimitVal
	StreamsUsage         int
	StreamsInbound       rcmgr.LimitVal
	StreamsInboundUsage  int
	StreamsOutbound      rcmgr.LimitVal
	StreamsOutboundUsage int
}

func (u ResourceLimitsAndUsage) ToResourceLimits() rcmgr.ResourceLimits {
	return rcmgr.ResourceLimits{
		Memory:          u.Memory,
		FD:              u.FD,
		Conns:           u.Conns,
		ConnsInbound:    u.ConnsInbound,
		ConnsOutbound:   u.ConnsOutbound,
		Streams:         u.Streams,
		StreamsInbound:  u.StreamsInbound,
		StreamsOutbound: u.StreamsOutbound,
	}
}

type LimitsConfigAndUsage struct {
	// This is duplicated from rcmgr.ResourceManagerStat but using ResourceLimitsAndUsage
	// instead of network.ScopeStat.
	System    ResourceLimitsAndUsage                 `json:",omitempty"`
	Transient ResourceLimitsAndUsage                 `json:",omitempty"`
	Services  map[string]ResourceLimitsAndUsage      `json:",omitempty"`
	Protocols map[protocol.ID]ResourceLimitsAndUsage `json:",omitempty"`
	Peers     map[peer.ID]ResourceLimitsAndUsage     `json:",omitempty"`
}

func (u LimitsConfigAndUsage) MarshalJSON() ([]byte, error) {
	// we want to marshal the encoded peer id
	encodedPeerMap := make(map[string]ResourceLimitsAndUsage, len(u.Peers))
	for p, v := range u.Peers {
		encodedPeerMap[p.String()] = v
	}

	type Alias LimitsConfigAndUsage
	return json.Marshal(&struct {
		*Alias
		Peers map[string]ResourceLimitsAndUsage `json:",omitempty"`
	}{
		Alias: (*Alias)(&u),
		Peers: encodedPeerMap,
	})
}

func (u LimitsConfigAndUsage) ToPartialLimitConfig() (result rcmgr.PartialLimitConfig) {
	result.System = u.System.ToResourceLimits()
	result.Transient = u.Transient.ToResourceLimits()

	result.Service = make(map[string]rcmgr.ResourceLimits, len(u.Services))
	for s, l := range u.Services {
		result.Service[s] = l.ToResourceLimits()
	}
	result.Protocol = make(map[protocol.ID]rcmgr.ResourceLimits, len(u.Protocols))
	for p, l := range u.Protocols {
		result.Protocol[p] = l.ToResourceLimits()
	}
	result.Peer = make(map[peer.ID]rcmgr.ResourceLimits, len(u.Peers))
	for p, l := range u.Peers {
		result.Peer[p] = l.ToResourceLimits()
	}

	return
}

func MergeLimitsAndStatsIntoLimitsConfigAndUsage(l rcmgr.ConcreteLimitConfig, stats rcmgr.ResourceManagerStat) LimitsConfigAndUsage {
	limits := l.ToPartialLimitConfig()

	return LimitsConfigAndUsage{
		System:    mergeResourceLimitsAndScopeStatToResourceLimitsAndUsage(limits.System, stats.System),
		Transient: mergeResourceLimitsAndScopeStatToResourceLimitsAndUsage(limits.Transient, stats.Transient),
		Services:  mergeLimitsAndStatsMapIntoLimitsConfigAndUsageMap(limits.Service, stats.Services),
		Protocols: mergeLimitsAndStatsMapIntoLimitsConfigAndUsageMap(limits.Protocol, stats.Protocols),
		Peers:     mergeLimitsAndStatsMapIntoLimitsConfigAndUsageMap(limits.Peer, stats.Peers),
	}
}

func mergeLimitsAndStatsMapIntoLimitsConfigAndUsageMap[K comparable](limits map[K]rcmgr.ResourceLimits, stats map[K]network.ScopeStat) map[K]ResourceLimitsAndUsage {
	r := make(map[K]ResourceLimitsAndUsage, maxInt(len(limits), len(stats)))
	for p, s := range stats {
		var l rcmgr.ResourceLimits
		if limits != nil {
			if rl, ok := limits[p]; ok {
				l = rl
			}
		}
		r[p] = mergeResourceLimitsAndScopeStatToResourceLimitsAndUsage(l, s)
	}
	for p, s := range limits {
		if _, ok := stats[p]; ok {
			continue // we already processed this element in the loop above
		}

		r[p] = mergeResourceLimitsAndScopeStatToResourceLimitsAndUsage(s, network.ScopeStat{})
	}
	return r
}

func maxInt(x, y int) int {
	if x > y {
		return x
	}
	return y
}

func mergeResourceLimitsAndScopeStatToResourceLimitsAndUsage(rl rcmgr.ResourceLimits, ss network.ScopeStat) ResourceLimitsAndUsage {
	return ResourceLimitsAndUsage{
		Memory:               rl.Memory,
		MemoryUsage:          ss.Memory,
		FD:                   rl.FD,
		FDUsage:              ss.NumFD,
		Conns:                rl.Conns,
		ConnsUsage:           ss.NumConnsOutbound + ss.NumConnsInbound,
		ConnsOutbound:        rl.ConnsOutbound,
		ConnsOutboundUsage:   ss.NumConnsOutbound,
		ConnsInbound:         rl.ConnsInbound,
		ConnsInboundUsage:    ss.NumConnsInbound,
		Streams:              rl.Streams,
		StreamsUsage:         ss.NumStreamsOutbound + ss.NumConnsInbound,
		StreamsOutbound:      rl.StreamsOutbound,
		StreamsOutboundUsage: ss.NumConnsOutbound,
		StreamsInbound:       rl.StreamsInbound,
		StreamsInboundUsage:  ss.NumConnsInbound,
	}
}

type ResourceInfos []ResourceInfo

type ResourceInfo struct {
	ScopeName    string
	LimitName    string
	LimitValue   rcmgr.LimitVal64
	CurrentUsage int64
}

// LimitConfigsToInfo gets limits and stats and generates a list of scopes and limits to be printed.
func LimitConfigsToInfo(stats LimitsConfigAndUsage) ResourceInfos {
	result := ResourceInfos{}

	result = append(result, resourceLimitsAndUsageToResourceInfo(config.ResourceMgrSystemScope, stats.System)...)
	result = append(result, resourceLimitsAndUsageToResourceInfo(config.ResourceMgrTransientScope, stats.Transient)...)

	for i, s := range stats.Services {
		result = append(result, resourceLimitsAndUsageToResourceInfo(
			config.ResourceMgrServiceScopePrefix+i,
			s,
		)...)
	}

	for i, p := range stats.Protocols {
		result = append(result, resourceLimitsAndUsageToResourceInfo(
			config.ResourceMgrProtocolScopePrefix+string(i),
			p,
		)...)
	}

	for i, p := range stats.Peers {
		result = append(result, resourceLimitsAndUsageToResourceInfo(
			config.ResourceMgrPeerScopePrefix+i.Pretty(),
			p,
		)...)
	}

	return result
}

const (
	limitNameMemory          = "Memory"
	limitNameFD              = "FD"
	limitNameConns           = "Conns"
	limitNameConnsInbound    = "ConnsInbound"
	limitNameConnsOutbound   = "ConnsOutbound"
	limitNameStreams         = "Streams"
	limitNameStreamsInbound  = "StreamsInbound"
	limitNameStreamsOutbound = "StreamsOutbound"
)

var limits = []string{
	limitNameMemory,
	limitNameFD,
	limitNameConns,
	limitNameConnsInbound,
	limitNameConnsOutbound,
	limitNameStreams,
	limitNameStreamsInbound,
	limitNameStreamsOutbound,
}

func resourceLimitsAndUsageToResourceInfo(scopeName string, stats ResourceLimitsAndUsage) ResourceInfos {
	result := ResourceInfos{}
	for _, l := range limits {
		ri := ResourceInfo{
			ScopeName: scopeName,
		}
		switch l {
		case limitNameMemory:
			ri.LimitName = limitNameMemory
			ri.LimitValue = stats.Memory
			ri.CurrentUsage = stats.MemoryUsage
		case limitNameFD:
			ri.LimitName = limitNameFD
			ri.LimitValue = rcmgr.LimitVal64(stats.FD)
			ri.CurrentUsage = int64(stats.FDUsage)
		case limitNameConns:
			ri.LimitName = limitNameConns
			ri.LimitValue = rcmgr.LimitVal64(stats.Conns)
			ri.CurrentUsage = int64(stats.ConnsUsage)
		case limitNameConnsInbound:
			ri.LimitName = limitNameConnsInbound
			ri.LimitValue = rcmgr.LimitVal64(stats.ConnsInbound)
			ri.CurrentUsage = int64(stats.ConnsInboundUsage)
		case limitNameConnsOutbound:
			ri.LimitName = limitNameConnsOutbound
			ri.LimitValue = rcmgr.LimitVal64(stats.ConnsOutbound)
			ri.CurrentUsage = int64(stats.ConnsOutboundUsage)
		case limitNameStreams:
			ri.LimitName = limitNameStreams
			ri.LimitValue = rcmgr.LimitVal64(stats.Streams)
			ri.CurrentUsage = int64(stats.StreamsUsage)
		case limitNameStreamsInbound:
			ri.LimitName = limitNameStreamsInbound
			ri.LimitValue = rcmgr.LimitVal64(stats.StreamsInbound)
			ri.CurrentUsage = int64(stats.StreamsInboundUsage)
		case limitNameStreamsOutbound:
			ri.LimitName = limitNameStreamsOutbound
			ri.LimitValue = rcmgr.LimitVal64(stats.StreamsOutbound)
			ri.CurrentUsage = int64(stats.StreamsOutboundUsage)
		}

		if ri.LimitValue == rcmgr.Unlimited64 || ri.LimitValue == rcmgr.DefaultLimit64 {
			// ignore unlimited and unset limits to remove noise from output.
			continue
		}

		result = append(result, ri)
	}

	return result
}

func ensureConnMgrMakeSenseVsResourceMgr(concreteLimits rcmgr.ConcreteLimitConfig, cfg config.SwarmConfig) error {
	if cfg.ConnMgr.Type.WithDefault(config.DefaultConnMgrType) == "none" || len(cfg.ResourceMgr.Allowlist) != 0 {
		// no connmgr OR
		// If an allowlist is set, a user may be enacting some form of DoS defense.
		// We don't want want to modify the System.ConnsInbound in that case for example
		// as it may make sense for it to be (and stay) as "blockAll"
		// so that only connections within the allowlist of multiaddrs get established.
		return nil
	}

	rcm := concreteLimits.ToPartialLimitConfig()

	highWater := cfg.ConnMgr.HighWater.WithDefault(config.DefaultConnMgrHighWater)
	if (rcm.System.Conns > rcmgr.DefaultLimit || rcm.System.Conns == rcmgr.BlockAllLimit) && int64(rcm.System.Conns) <= highWater {
		// nolint
		errMsg := `unable to initialize libp2p due to conflicting resource manager limit configuration. resource manager System.Conns (%d) must be bigger than ConnMgr.HighWater (%d)
		See: https://github.com/ipfs/kubo/blob/master/docs/libp2p-resource-management.md#how-does-the-resource-manager-resourcemgr-relate-to-the-connection-manager-connmgr
		`
		return fmt.Errorf(errMsg, rcm.System.Conns, highWater)
	}
	if (rcm.System.ConnsInbound > rcmgr.DefaultLimit || rcm.System.ConnsInbound == rcmgr.BlockAllLimit) && int64(rcm.System.ConnsInbound) <= highWater {
		// nolint
		errMsg := `unable to initialize libp2p due to conflicting resource manager limit configuration. resource manager System.ConnsInbound (%d) must be bigger than ConnMgr.HighWater (%d)
		See: https://github.com/ipfs/kubo/blob/master/docs/libp2p-resource-management.md#how-does-the-resource-manager-resourcemgr-relate-to-the-connection-manager-connmgr
		`
		return fmt.Errorf(errMsg, rcm.System.ConnsInbound, highWater)
	}
	if rcm.System.Streams > rcmgr.DefaultLimit || rcm.System.Streams == rcmgr.BlockAllLimit && int64(rcm.System.Streams) <= highWater {
		// nolint
		errMsg := `
		unable to initialize libp2p due to conflicting resource manager limit configuration.
		resource manager System.Streams (%d) must be bigger than ConnMgr.HighWater (%d)
		See: https://github.com/ipfs/kubo/blob/master/docs/libp2p-resource-management.md#how-does-the-resource-manager-resourcemgr-relate-to-the-connection-manager-connmgr
		`
		return fmt.Errorf(errMsg, rcm.System.Streams, highWater)
	}
	if (rcm.System.StreamsInbound > rcmgr.DefaultLimit || rcm.System.StreamsInbound == rcmgr.BlockAllLimit) && int64(rcm.System.StreamsInbound) <= highWater {
		// nolint
		errMsg := `
		Unable to initialize libp2p due to conflicting resource manager limit configuration.
		resource manager System.StreamsInbound (%d) must be bigger than ConnMgr.HighWater (%d)
		See: https://github.com/ipfs/kubo/blob/master/docs/libp2p-resource-management.md#how-does-the-resource-manager-resourcemgr-relate-to-the-connection-manager-connmgr
		`
		return fmt.Errorf(errMsg, rcm.System.StreamsInbound, highWater)
	}
	return nil
}

// rcmgr_defaults.go
var infiniteResourceLimits = rcmgr.InfiniteLimits.ToPartialLimitConfig().System

// This file defines implicit limit defaults used when Swarm.ResourceMgr.Enabled

// createDefaultLimitConfig creates LimitConfig to pass to libp2p's resource manager.
// The defaults follow the documentation in docs/libp2p-resource-management.md.
// Any changes in the logic here should be reflected there.
func createDefaultLimitConfig(cfg config.SwarmConfig) (limitConfig rcmgr.ConcreteLimitConfig, logMessageForStartup string, err error) {
	maxMemoryDefaultString := humanize.Bytes(uint64(memory.TotalMemory()) / 2)
	maxMemoryString := cfg.ResourceMgr.MaxMemory.WithDefault(maxMemoryDefaultString)
	maxMemory, err := humanize.ParseBytes(maxMemoryString)
	if err != nil {
		return rcmgr.ConcreteLimitConfig{}, "", err
	}

	maxMemoryMB := maxMemory / (1024 * 1024)
	maxFD := int(cfg.ResourceMgr.MaxFileDescriptors.WithDefault(int64(fd.GetNumFDs()) / 2))

	// At least as of 2023-01-25, it's possible to open a connection that
	// doesn't ask for any memory usage with the libp2p Resource Manager/Accountant
	// (see https://github.com/libp2p/go-libp2p/issues/2010#issuecomment-1404280736).
	// As a result, we can't curretly rely on Memory limits to full protect us.
	// Until https://github.com/libp2p/go-libp2p/issues/2010 is addressed,
	// we take a proxy now of restricting to 1 inbound connection per MB.
	// Note: this is more generous than go-libp2p's default autoscaled limits which do
	// 64 connections per 1GB
	// (see https://github.com/libp2p/go-libp2p/blob/master/p2p/host/resource-manager/limit_defaults.go#L357 ).
	systemConnsInbound := int(1 * maxMemoryMB)

	partialLimits := rcmgr.PartialLimitConfig{
		System: rcmgr.ResourceLimits{
			Memory: rcmgr.LimitVal64(maxMemory),
			FD:     rcmgr.LimitVal(maxFD),

			Conns:         rcmgr.Unlimited,
			ConnsInbound:  rcmgr.LimitVal(systemConnsInbound),
			ConnsOutbound: rcmgr.Unlimited,

			Streams:         rcmgr.Unlimited,
			StreamsOutbound: rcmgr.Unlimited,
			StreamsInbound:  rcmgr.Unlimited,
		},

		// Transient connections won't cause any memory to be accounted for by the resource manager/accountant.
		// Only established connections do.
		// As a result, we can't rely on System.Memory to protect us from a bunch of transient connection being opened.
		// We limit the same values as the System scope, but only allow the Transient scope to take 25% of what is allowed for the System scope.
		Transient: rcmgr.ResourceLimits{
			Memory: rcmgr.LimitVal64(maxMemory / 4),
			FD:     rcmgr.LimitVal(maxFD / 4),

			Conns:         rcmgr.Unlimited,
			ConnsInbound:  rcmgr.LimitVal(systemConnsInbound / 4),
			ConnsOutbound: rcmgr.Unlimited,

			Streams:         rcmgr.Unlimited,
			StreamsInbound:  rcmgr.Unlimited,
			StreamsOutbound: rcmgr.Unlimited,
		},

		// Lets get out of the way of the allow list functionality.
		// If someone specified "Swarm.ResourceMgr.Allowlist" we should let it go through.
		AllowlistedSystem: infiniteResourceLimits,

		AllowlistedTransient: infiniteResourceLimits,

		// Keep it simple by not having Service, ServicePeer, Protocol, ProtocolPeer, Conn, or Stream limits.
		ServiceDefault: infiniteResourceLimits,

		ServicePeerDefault: infiniteResourceLimits,

		ProtocolDefault: infiniteResourceLimits,

		ProtocolPeerDefault: infiniteResourceLimits,

		Conn: infiniteResourceLimits,

		Stream: infiniteResourceLimits,

		// Limit the resources consumed by a peer.
		// This doesn't protect us against intentional DoS attacks since an attacker can easily spin up multiple peers.
		// We specify this limit against unintentional DoS attacks (e.g., a peer has a bug and is sending too much traffic intentionally).
		// In that case we want to keep that peer's resource consumption contained.
		// To keep this simple, we only constrain inbound connections and streams.
		PeerDefault: rcmgr.ResourceLimits{
			Memory:          rcmgr.Unlimited64,
			FD:              rcmgr.Unlimited,
			Conns:           rcmgr.Unlimited,
			ConnsInbound:    rcmgr.DefaultLimit,
			ConnsOutbound:   rcmgr.Unlimited,
			Streams:         rcmgr.Unlimited,
			StreamsInbound:  rcmgr.DefaultLimit,
			StreamsOutbound: rcmgr.Unlimited,
		},
	}

	scalingLimitConfig := rcmgr.DefaultLimits
	libp2p.SetDefaultServiceLimits(&scalingLimitConfig)

	// Anything set above in partialLimits that had a value of rcmgr.DefaultLimit will be overridden.
	// Anything in scalingLimitConfig that wasn't defined in partialLimits above will be added (e.g., libp2p's default service limits).
	partialLimits = partialLimits.Build(scalingLimitConfig.Scale(int64(maxMemory), maxFD)).ToPartialLimitConfig()

	// Simple checks to overide autoscaling ensuring limits make sense versus the connmgr values.
	// There are ways to break this, but this should catch most problems already.
	// We might improve this in the future.
	// See: https://github.com/ipfs/kubo/issues/9545
	if partialLimits.System.ConnsInbound > rcmgr.DefaultLimit && cfg.ConnMgr.Type.WithDefault(config.DefaultConnMgrType) != "none" {
		maxInboundConns := int64(partialLimits.System.ConnsInbound)
		if connmgrHighWaterTimesTwo := cfg.ConnMgr.HighWater.WithDefault(config.DefaultConnMgrHighWater) * 2; maxInboundConns < connmgrHighWaterTimesTwo {
			maxInboundConns = connmgrHighWaterTimesTwo
		}

		if maxInboundConns < config.DefaultResourceMgrMinInboundConns {
			maxInboundConns = config.DefaultResourceMgrMinInboundConns
		}

		// Scale System.StreamsInbound as well, but use the existing ratio of StreamsInbound to ConnsInbound
		if partialLimits.System.StreamsInbound > rcmgr.DefaultLimit {
			partialLimits.System.StreamsInbound = rcmgr.LimitVal(maxInboundConns * int64(partialLimits.System.StreamsInbound) / int64(partialLimits.System.ConnsInbound))
		}
		partialLimits.System.ConnsInbound = rcmgr.LimitVal(maxInboundConns)
	}

	msg := fmt.Sprintf(`Computed default go-libp2p Resource Manager limits based on: - 'Swarm.ResourceMgr.MaxMemory': %q
	- 'Swarm.ResourceMgr.MaxFileDescriptors': %d Theses can be inspected with 'tvbase swarm resources'.`, maxMemoryString, maxFD)

	// We already have a complete value thus pass in an empty ConcreteLimitConfig.
	return partialLimits.Build(rcmgr.ConcreteLimitConfig{}), msg, nil
}

// rcmgr_metrics.go
func mustRegister(c prometheus.Collector) {
	err := prometheus.Register(c)
	are := prometheus.AlreadyRegisteredError{}
	if errors.As(err, &are) {
		return
	}
	if err != nil {
		panic(err)
	}
}

func createRcmgrMetrics() rcmgr.MetricsReporter {
	const (
		direction = "direction"
		usesFD    = "usesFD"
		protocol  = "protocol"
		service   = "service"
	)

	connAllowed := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "libp2p_rcmgr_conns_allowed_total",
			Help: "allowed connections",
		},
		[]string{direction, usesFD},
	)
	mustRegister(connAllowed)

	connBlocked := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "libp2p_rcmgr_conns_blocked_total",
			Help: "blocked connections",
		},
		[]string{direction, usesFD},
	)
	mustRegister(connBlocked)

	streamAllowed := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "libp2p_rcmgr_streams_allowed_total",
			Help: "allowed streams",
		},
		[]string{direction},
	)
	mustRegister(streamAllowed)

	streamBlocked := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "libp2p_rcmgr_streams_blocked_total",
			Help: "blocked streams",
		},
		[]string{direction},
	)
	mustRegister(streamBlocked)

	peerAllowed := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "libp2p_rcmgr_peers_allowed_total",
		Help: "allowed peers",
	})
	mustRegister(peerAllowed)

	peerBlocked := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "libp2p_rcmgr_peer_blocked_total",
		Help: "blocked peers",
	})
	mustRegister(peerBlocked)

	protocolAllowed := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "libp2p_rcmgr_protocols_allowed_total",
			Help: "allowed streams attached to a protocol",
		},
		[]string{protocol},
	)
	mustRegister(protocolAllowed)

	protocolBlocked := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "libp2p_rcmgr_protocols_blocked_total",
			Help: "blocked streams attached to a protocol",
		},
		[]string{protocol},
	)
	mustRegister(protocolBlocked)

	protocolPeerBlocked := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "libp2p_rcmgr_protocols_for_peer_blocked_total",
			Help: "blocked streams attached to a protocol for a specific peer",
		},
		[]string{protocol},
	)
	mustRegister(protocolPeerBlocked)

	serviceAllowed := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "libp2p_rcmgr_services_allowed_total",
			Help: "allowed streams attached to a service",
		},
		[]string{service},
	)
	mustRegister(serviceAllowed)

	serviceBlocked := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "libp2p_rcmgr_services_blocked_total",
			Help: "blocked streams attached to a service",
		},
		[]string{service},
	)
	mustRegister(serviceBlocked)

	servicePeerBlocked := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "libp2p_rcmgr_service_for_peer_blocked_total",
			Help: "blocked streams attached to a service for a specific peer",
		},
		[]string{service},
	)
	mustRegister(servicePeerBlocked)

	memoryAllowed := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "libp2p_rcmgr_memory_allocations_allowed_total",
		Help: "allowed memory allocations",
	})
	mustRegister(memoryAllowed)

	memoryBlocked := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "libp2p_rcmgr_memory_allocations_blocked_total",
		Help: "blocked memory allocations",
	})
	mustRegister(memoryBlocked)

	return rcmgrMetrics{
		connAllowed,
		connBlocked,
		streamAllowed,
		streamBlocked,
		peerAllowed,
		peerBlocked,
		protocolAllowed,
		protocolBlocked,
		protocolPeerBlocked,
		serviceAllowed,
		serviceBlocked,
		servicePeerBlocked,
		memoryAllowed,
		memoryBlocked,
	}
}

// Failsafe to ensure interface from go-libp2p-resource-manager is implemented
var _ rcmgr.MetricsReporter = rcmgrMetrics{}

type rcmgrMetrics struct {
	connAllowed         *prometheus.CounterVec
	connBlocked         *prometheus.CounterVec
	streamAllowed       *prometheus.CounterVec
	streamBlocked       *prometheus.CounterVec
	peerAllowed         prometheus.Counter
	peerBlocked         prometheus.Counter
	protocolAllowed     *prometheus.CounterVec
	protocolBlocked     *prometheus.CounterVec
	protocolPeerBlocked *prometheus.CounterVec
	serviceAllowed      *prometheus.CounterVec
	serviceBlocked      *prometheus.CounterVec
	servicePeerBlocked  *prometheus.CounterVec
	memoryAllowed       prometheus.Counter
	memoryBlocked       prometheus.Counter
}

func getDirection(d network.Direction) string {
	switch d {
	default:
		return ""
	case network.DirInbound:
		return "inbound"
	case network.DirOutbound:
		return "outbound"
	}
}

func (r rcmgrMetrics) AllowConn(dir network.Direction, usefd bool) {
	r.connAllowed.WithLabelValues(getDirection(dir), strconv.FormatBool(usefd)).Inc()
}

func (r rcmgrMetrics) BlockConn(dir network.Direction, usefd bool) {
	r.connBlocked.WithLabelValues(getDirection(dir), strconv.FormatBool(usefd)).Inc()
}

func (r rcmgrMetrics) AllowStream(_ peer.ID, dir network.Direction) {
	r.streamAllowed.WithLabelValues(getDirection(dir)).Inc()
}

func (r rcmgrMetrics) BlockStream(_ peer.ID, dir network.Direction) {
	r.streamBlocked.WithLabelValues(getDirection(dir)).Inc()
}

func (r rcmgrMetrics) AllowPeer(_ peer.ID) {
	r.peerAllowed.Inc()
}

func (r rcmgrMetrics) BlockPeer(_ peer.ID) {
	r.peerBlocked.Inc()
}

func (r rcmgrMetrics) AllowProtocol(proto protocol.ID) {
	r.protocolAllowed.WithLabelValues(string(proto)).Inc()
}

func (r rcmgrMetrics) BlockProtocol(proto protocol.ID) {
	r.protocolBlocked.WithLabelValues(string(proto)).Inc()
}

func (r rcmgrMetrics) BlockProtocolPeer(proto protocol.ID, _ peer.ID) {
	r.protocolPeerBlocked.WithLabelValues(string(proto)).Inc()
}

func (r rcmgrMetrics) AllowService(svc string) {
	r.serviceAllowed.WithLabelValues(svc).Inc()
}

func (r rcmgrMetrics) BlockService(svc string) {
	r.serviceBlocked.WithLabelValues(svc).Inc()
}

func (r rcmgrMetrics) BlockServicePeer(svc string, _ peer.ID) {
	r.servicePeerBlocked.WithLabelValues(svc).Inc()
}

func (r rcmgrMetrics) AllowMemory(_ int) {
	r.memoryAllowed.Inc()
}

func (r rcmgrMetrics) BlockMemory(_ int) {
	r.memoryBlocked.Inc()
}

// rcmgr_logging.go
type loggingResourceManager struct {
	clock       clock.Clock
	logger      *zap.SugaredLogger
	delegate    network.ResourceManager
	logInterval time.Duration

	mut               sync.Mutex
	limitExceededErrs map[string]int
}

type loggingScope struct {
	logger    *zap.SugaredLogger
	delegate  network.ResourceScope
	countErrs func(error)
}

var _ network.ResourceManager = (*loggingResourceManager)(nil)
var _ rcmgr.ResourceManagerState = (*loggingResourceManager)(nil)

func (n *loggingResourceManager) start(ctx context.Context) {
	logInterval := n.logInterval
	if logInterval == 0 {
		logInterval = 10 * time.Second
	}
	ticker := n.clock.Ticker(logInterval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				n.mut.Lock()
				errs := n.limitExceededErrs
				n.limitExceededErrs = make(map[string]int)

				for e, count := range errs {
					n.logger.Warnf("Protected from exceeding resource limits %d times.  libp2p message: %q.", count, e)
				}

				if len(errs) != 0 {
					n.logger.Warnf("Learn more about potential actions to take at: https://github.com/ipfs/kubo/blob/master/docs/libp2p-resource-management.md")
				}

				n.mut.Unlock()
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (n *loggingResourceManager) countErrs(err error) {
	if errors.Is(err, network.ErrResourceLimitExceeded) {
		n.mut.Lock()
		if n.limitExceededErrs == nil {
			n.limitExceededErrs = make(map[string]int)
		}

		// we need to unwrap the error to get the limit scope and the kind of reached limit
		eout := errors.Unwrap(err)
		if eout != nil {
			n.limitExceededErrs[eout.Error()]++
		}

		n.mut.Unlock()
	}
}

func (n *loggingResourceManager) ViewSystem(f func(network.ResourceScope) error) error {
	return n.delegate.ViewSystem(f)
}
func (n *loggingResourceManager) ViewTransient(f func(network.ResourceScope) error) error {
	return n.delegate.ViewTransient(func(s network.ResourceScope) error {
		return f(&loggingScope{logger: n.logger, delegate: s, countErrs: n.countErrs})
	})
}
func (n *loggingResourceManager) ViewService(svc string, f func(network.ServiceScope) error) error {
	return n.delegate.ViewService(svc, func(s network.ServiceScope) error {
		return f(&loggingScope{logger: n.logger, delegate: s, countErrs: n.countErrs})
	})
}
func (n *loggingResourceManager) ViewProtocol(p protocol.ID, f func(network.ProtocolScope) error) error {
	return n.delegate.ViewProtocol(p, func(s network.ProtocolScope) error {
		return f(&loggingScope{logger: n.logger, delegate: s, countErrs: n.countErrs})
	})
}
func (n *loggingResourceManager) ViewPeer(p peer.ID, f func(network.PeerScope) error) error {
	return n.delegate.ViewPeer(p, func(s network.PeerScope) error {
		return f(&loggingScope{logger: n.logger, delegate: s, countErrs: n.countErrs})
	})
}
func (n *loggingResourceManager) OpenConnection(dir network.Direction, usefd bool, remote ma.Multiaddr) (network.ConnManagementScope, error) {
	connMgmtScope, err := n.delegate.OpenConnection(dir, usefd, remote)
	n.countErrs(err)
	return connMgmtScope, err
}
func (n *loggingResourceManager) OpenStream(p peer.ID, dir network.Direction) (network.StreamManagementScope, error) {
	connMgmtScope, err := n.delegate.OpenStream(p, dir)
	n.countErrs(err)
	return connMgmtScope, err
}
func (n *loggingResourceManager) Close() error {
	return n.delegate.Close()
}

func (n *loggingResourceManager) ListServices() []string {
	rapi, ok := n.delegate.(rcmgr.ResourceManagerState)
	if !ok {
		return nil
	}

	return rapi.ListServices()
}
func (n *loggingResourceManager) ListProtocols() []protocol.ID {
	rapi, ok := n.delegate.(rcmgr.ResourceManagerState)
	if !ok {
		return nil
	}

	return rapi.ListProtocols()
}
func (n *loggingResourceManager) ListPeers() []peer.ID {
	rapi, ok := n.delegate.(rcmgr.ResourceManagerState)
	if !ok {
		return nil
	}

	return rapi.ListPeers()
}

func (n *loggingResourceManager) Stat() rcmgr.ResourceManagerStat {
	rapi, ok := n.delegate.(rcmgr.ResourceManagerState)
	if !ok {
		return rcmgr.ResourceManagerStat{}
	}

	return rapi.Stat()
}

func (s *loggingScope) ReserveMemory(size int, prio uint8) error {
	err := s.delegate.ReserveMemory(size, prio)
	s.countErrs(err)
	return err
}
func (s *loggingScope) ReleaseMemory(size int) {
	s.delegate.ReleaseMemory(size)
}
func (s *loggingScope) Stat() network.ScopeStat {
	return s.delegate.Stat()
}
func (s *loggingScope) BeginSpan() (network.ResourceScopeSpan, error) {
	return s.delegate.BeginSpan()
}
func (s *loggingScope) Done() {
	s.delegate.(network.ResourceScopeSpan).Done()
}
func (s *loggingScope) Name() string {
	return s.delegate.(network.ServiceScope).Name()
}
func (s *loggingScope) Protocol() protocol.ID {
	return s.delegate.(network.ProtocolScope).Protocol()
}
func (s *loggingScope) Peer() peer.ID {
	return s.delegate.(network.PeerScope).Peer()
}
func (s *loggingScope) PeerScope() network.PeerScope {
	return s.delegate.(network.PeerScope)
}
func (s *loggingScope) SetPeer(p peer.ID) error {
	err := s.delegate.(network.ConnManagementScope).SetPeer(p)
	s.countErrs(err)
	return err
}
func (s *loggingScope) ProtocolScope() network.ProtocolScope {
	return s.delegate.(network.ProtocolScope)
}
func (s *loggingScope) SetProtocol(proto protocol.ID) error {
	err := s.delegate.(network.StreamManagementScope).SetProtocol(proto)
	s.countErrs(err)
	return err
}
func (s *loggingScope) ServiceScope() network.ServiceScope {
	return s.delegate.(network.ServiceScope)
}
func (s *loggingScope) SetService(srv string) error {
	err := s.delegate.(network.StreamManagementScope).SetService(srv)
	s.countErrs(err)
	return err
}
func (s *loggingScope) Limit() rcmgr.Limit {
	return s.delegate.(rcmgr.ResourceScopeLimiter).Limit()
}
func (s *loggingScope) SetLimit(limit rcmgr.Limit) {
	s.delegate.(rcmgr.ResourceScopeLimiter).SetLimit(limit)
}
