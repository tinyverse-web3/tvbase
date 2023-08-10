package common

import (
	"context"
	"time"

	kaddht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/tinyverse-web3/tvbase/common/config"
	"github.com/tinyverse-web3/tvbase/common/db"
	tvPeer "github.com/tinyverse-web3/tvbase/common/peer"
	tvProtocol "github.com/tinyverse-web3/tvbase/common/protocol"
	dkvspb "github.com/tinyverse-web3/tvbase/dkvs/pb"
	"go.opentelemetry.io/otel/trace"
)

type DmsgService interface {
	Start() error
	Stop() error
}

type DkvsService interface {
	Put(key string, val []byte, pubkey []byte, issuetime uint64, ttl uint64, sig []byte) error
	// return: value, pubkey, issuetime, ttl, signature, err
	Get(key string) ([]byte, []byte, uint64, uint64, []byte, error)
	GetRecord(key string) (*dkvspb.DkvsRecord, error)
	FastGetRecord(key string) (*dkvspb.DkvsRecord, error)
	Has(key string) bool
	CheckTransferPara(key string, value1 []byte, pubkey1 []byte, sig1 []byte, value2 []byte, pubkey2 []byte, issuetime uint64, ttl uint64, sig2 []byte, txcert *dkvspb.Cert) error
	TransferKey(key string, value1 []byte, pubkey1 []byte, sig1 []byte, value2 []byte, pubkey2 []byte, issuetime uint64, ttl uint64, sig2 []byte, txcert *dkvspb.Cert) error
	IsPublicService(sn string, pubkey []byte) bool
	IsChildPubkey(child []byte, parent []byte) bool
	IsApprovedService(sn string) bool
	FindPeersByKey(ctx context.Context, key string, timeout time.Duration) []peer.AddrInfo
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
	GetDkvsService() DkvsService
	GetTraceSpan() trace.Span
	TraceSpan(componentName string, spanName string, options ...any) error
	SetTracerStatus(err error)
	GetAvailableServicePeerList(key string) ([]peer.ID, error)
	GetAvailableLightPeerList(key string) ([]peer.ID, error)
	RegistConnectedCallback(callback tvPeer.ConnectCallback)
	RegistNotConnectedCallback(callback tvPeer.ConnectCallback)
	GetIsRendezvous() bool
	GetRendezvousChan() chan bool
}
