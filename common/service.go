package common

import (
	"context"

	kaddht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/tinyverse-web3/tvbase/common/config"
	tvPeer "github.com/tinyverse-web3/tvbase/common/peer"
	tvProtocol "github.com/tinyverse-web3/tvbase/common/protocol"
	db "github.com/tinyverse-web3/tvutil/db"
	"go.opentelemetry.io/otel/trace"
)

type DmsgService interface {
	Start() error
	Stop() error
}

type TraceSpanCallback func(ctx context.Context)

type NoArgCallback func() error

type TvBaseService interface {
	DiscoverRendezvousPeers()
	GetServicePeerList() tvPeer.PeerInfoList
	GetLightPeerList() tvPeer.PeerInfoList
	GetConfig() *config.NodeConfig
	GetNodeInfoService() *tvProtocol.NodeInfoService
	GetDht() *kaddht.IpfsDHT
	GetCtx() context.Context
	GetHost() host.Host
	GetDhtDatabase() db.Datastore
	GetTraceSpan() trace.Span
	TraceSpan(componentName string, spanName string, options ...any) error
	SetTracerStatus(err error)
	GetAvailableServicePeerList(key string) ([]peer.ID, error)
	GetAvailableLightPeerList(key string) ([]peer.ID, error)
	RegistConnectedCallback(callback tvPeer.ConnectCallback)
	RegistNotConnectedCallback(callback tvPeer.ConnectCallback)

	// RegistNetReachabilityChanged(NoArgCallback) error
	// ConnectBootstrapNode()
	// GetDhtProtocolPrefix() string
}
