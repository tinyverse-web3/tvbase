package main

import (
	"bytes"
	"encoding/base64"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/tinyverse-web3/tvbase/common/config"
)

func setTestEnv(cfg *config.TvbaseConfig) error {
	cfg.SetLocalNet(true)
	cfg.SetMdns(false)
	cfg.SetDhtProtocolPrefix("/tvnode_test")
	cfg.Mode = nodeMode
	switch cfg.Mode {
	case config.LightMode:
		cfg.Network.ListenAddrs = []string{
			"/ip4/0.0.0.0/udp/" + "0" + "/quic",
			"/ip6/::/udp/" + "0" + "/quic",
			"/ip4/0.0.0.0/tcp/" + "0",
			"/ip6/::/tcp/" + "0",
		}
	case config.ServiceMode:
		cfg.Network.ListenAddrs = []string{
			"/ip4/0.0.0.0/udp/" + "9000" + "/quic",
			"/ip6/::/udp/" + "9000" + "/quic",
			"/ip4/0.0.0.0/tcp/" + "9000",
			"/ip6/::/tcp/" + "9000",
		}
	}

	if pkseed != nil {
		seedBytes := []byte(*pkseed)
		seedReader := bytes.NewReader(seedBytes)
		privateKey, publicKey, err := crypto.GenerateKeyPairWithReader(crypto.Ed25519, 0, seedReader)
		if err != nil {
			return err
		}
		privateKeyData, err := crypto.MarshalPrivateKey(privateKey)
		if err != nil {
			return err
		}
		cfg.Identity.PrivKey = base64.StdEncoding.EncodeToString(privateKeyData)

		privateKeyStr := base64.StdEncoding.EncodeToString(privateKeyData)
		publicKeyData, _ := crypto.MarshalPublicKey(publicKey)
		publicKeyStr := base64.StdEncoding.EncodeToString(publicKeyData)
		peerId, _ := peer.IDFromPublicKey(publicKey)
		logger.Infof("test identity:\nprivateKey: %s\npublicKey: %s\npeerId: %s", privateKeyStr, publicKeyStr, peerId.Pretty())
	}

	cfg.ClearBootstrapPeers()
	if bootpeer != nil {
		cfg.AddBootstrapPeer(*bootpeer)
	}

	return nil
}
