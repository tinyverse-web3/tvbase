package define

import (
	kaddht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	tvPeer "github.com/tinyverse-web3/tvbase/common/peer"
)

type DiagnosisInfo struct {
	Host                   host.Host
	Dht                    *kaddht.IpfsDHT
	IsRendezvous           bool
	IsDiscoverRendzvousing bool
	ServicePeerList        tvPeer.PeerInfoList
	LightPeerList          tvPeer.PeerInfoList
	NetworkPeers           []peer.ID
}
