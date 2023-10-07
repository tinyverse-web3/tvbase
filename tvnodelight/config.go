package main

import (
	"encoding/base64"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/tinyverse-web3/tvbase/common/config"
	"github.com/tinyverse-web3/tvbase/common/identity"
)

func initConfig() (*config.TvbaseConfig, error) {
	ret := config.NewDefaultTvbaseConfig()
	ret.InitMode(config.LightMode)

	privateKey, prikeyHex, err := identity.GenIdenity()
	if err != nil {
		return nil, err
	}
	privateKeyData, _ := crypto.MarshalPrivateKey(privateKey)
	privateKeyStr := base64.StdEncoding.EncodeToString(privateKeyData)
	publicKey := privateKey.GetPublic()
	publicKeyData, _ := crypto.MarshalPublicKey(publicKey)
	publicKeyStr := base64.StdEncoding.EncodeToString(publicKeyData)
	peerId, _ := peer.IDFromPublicKey(publicKey)
	logger.Infof("\nprivateKey: %s\npublicKey: %s\npeerId: %s", privateKeyStr, publicKeyStr, peerId.Pretty())
	ret.Identity.PrivKey = prikeyHex
	return ret, nil
}

func setTestEnv(cfg *config.TvbaseConfig) {
	cfg.SetLocalNet(true)
	cfg.SetMdns(false)
	cfg.SetDhtProtocolPrefix("/tvnode_test")
	cfg.SetPrivKeyStr("CAESQDEY5DwsYzpOVAikef0L5I8ijcdE2X1LYlC/KAc9CotperesYgBO3dXwgmUgLRUtFUu3eDw2J1CXKA/W9N9raZ4=")
	cfg.ClearBootstrapPeers()
	cfg.AddBootstrapPeer("/ip4/127.0.0.1/tcp/9000/p2p/12D3KooWJhbWTehsVzQm77FWozmvzPFsLmobb2GWMkSZq6DBkXRM")
	// cfg.AddBootstrapPeer("/ip4/192.168.1.102/tcp/9000/p2p/12D3KooWPThTtBAaC5vvnj6NE2iQSfuBHRUdtPweM6dER62R57R2")
	// cfg.AddBootstrapPeer("/ip4/192.168.1.109/tcp/9000/p2p/12D3KooWQvMGQWCRGdjtaFvqbdQ7qf8cw1x94hy1mWMvQovF6uAE")
}
