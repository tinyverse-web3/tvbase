package main

import (
	"crypto/ecdsa"

	"github.com/tinyverse-web3/mtv_go_utils/key"
)

func getKeyBySeed(seed string) (*ecdsa.PrivateKey, *ecdsa.PublicKey, error) {
	prikey, pubkey, err := key.GenerateEcdsaKey(seed)
	if err != nil {
		return nil, nil, err
	}
	return prikey, pubkey, nil
}
