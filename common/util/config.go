package util

import (
	"encoding/json"
	"os"

	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

// ParseBootstrapPeer parses a bootstrap list into a list of AddrInfos.
func ParseBootstrapPeers(addrs []string) ([]peer.AddrInfo, error) {
	maddrs := make([]ma.Multiaddr, len(addrs))
	for i, addr := range addrs {
		var err error
		maddrs[i], err = ma.NewMultiaddr(addr)
		if err != nil {
			return nil, err
		}
	}
	return peer.AddrInfosFromP2pAddrs(maddrs...)
}

func LoadConfig(cfg any, filePath string) error {
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
