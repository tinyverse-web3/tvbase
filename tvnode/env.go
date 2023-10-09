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
		cfg.SetPrivKeyStr("CAESQOVslnc19q6dtCdQlIO7dqp9DGsd7pzt3xERVboQ40d613YL+ED0VgGMAZ/eCRmlAa3hhTsphsqSjvRTMjJaKP0=")
		cfg.ClearBootstrapPeers()
		cfg.AddBootstrapPeer("/ip4/127.0.0.1/tcp/9000/p2p/12D3KooWFET1qH5xgeg3QrQm5NAMtvSJbKECH3AyBFQBghPZ8R2M")
	case config.ServiceMode:
		cfg.Network.ListenAddrs = []string{
			"/ip4/0.0.0.0/udp/" + "9000" + "/quic",
			"/ip6/::/udp/" + "9000" + "/quic",
			"/ip4/0.0.0.0/tcp/" + "9000",
			"/ip6/::/tcp/" + "9000",
		}
		cfg.SetPrivKeyStr("CAESQAo+pk645TCLemsAAPed3HuwOxCEnvTfp9IGJ7umqgE0UHXalXwaL8WagndTHAViieVzuOd0GJy1Bzp0elDZrAY=")
		cfg.ClearBootstrapPeers()
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

	return nil
}

/*
./tvnode -init -path "."
privateKey: CAESQAo+pk645TCLemsAAPed3HuwOxCEnvTfp9IGJ7umqgE0UHXalXwaL8WagndTHAViieVzuOd0GJy1Bzp0elDZrAY=
publicKey: CAESIFB12pV8Gi/FmoJ3UxwFYonlc7jndBictQc6dHpQ2awG
peerId: 12D3KooWFET1qH5xgeg3QrQm5NAMtvSJbKECH3AyBFQBghPZ8R2M

privateKey: CAESQOVslnc19q6dtCdQlIO7dqp9DGsd7pzt3xERVboQ40d613YL+ED0VgGMAZ/eCRmlAa3hhTsphsqSjvRTMjJaKP0=
publicKey: CAESINd2C/hA9FYBjAGf3gkZpQGt4YU7KYbKko70UzIyWij9
peerId: 12D3KooWQKSE15nnzRhEjC44JLGKJjCk7Zuqa8xKp8np8bXrUMKJ

privateKey: CAESQHXYP9L8tCrfSrNckCsEfzXVHoTIqN//NLoRTagS2+1dpd+Cq3yuARouN1vwqaBNIXJHUHgejf8/WEEqy3Oonqk=
publicKey: CAESIKXfgqt8rgEaLjdb8KmgTSFyR1B4Ho3/P1hBKstzqJ6p
peerId: 12D3KooWLys7VNJqikW41xmEiZczuf7PN5tJ4DKDGPbUE8c9Am2x

privateKey: CAESQD+OKGdG56I/1di6X0Q1jVgbM+a9qoEH871mMtiUOpu2sRYeDj1y172cSMUwI4Qwz1j9JYBVTVUaCvwYQc2ocTM=
publicKey: CAESILEWHg49cte9nEjFMCOEMM9Y/SWAVU1VGgr8GEHNqHEz
peerId: 12D3KooWMjdth95DnXU4rALF3TY3F9GUWgfVAGXRFsvUfKAVV75t

*/
