package peer

import (
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

type PeerInfo struct {
	PeerID        peer.ID
	ConnectStatus network.Connectedness
}
type PeerInfoList map[string]*PeerInfo

type ConnectCallback func(peerID peer.ID) error
