package main

import "github.com/tinyverse-web3/tvbase/common/config"

func setTestEnv(cfg *config.TvbaseConfig) {
	cfg.SetLocalNet(true)
	cfg.SetMdns(false)
	cfg.SetDhtProtocolPrefix("/tvnode_test")
	cfg.ClearBootstrapPeers()
	// cfg.AddBootstrapPeer("/ip4/192.168.1.102/tcp/9000/p2p/12D3KooWPThTtBAaC5vvnj6NE2iQSfuBHRUdtPweM6dER62R57R2")
}
