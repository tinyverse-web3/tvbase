package define

import (
	"github.com/libp2p/go-libp2p/core/peer"
	tvPeer "github.com/tinyverse-web3/tvbase/common/peer"
)

type DiagnosisInfo struct {
	IsRendezvous           bool
	IsDiscoverRendzvousing bool
	ServicePeerList        tvPeer.PeerInfoList
	LightPeerList          tvPeer.PeerInfoList
	NetworkPeers           []peer.ID
}
