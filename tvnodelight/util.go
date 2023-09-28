package main

import (
	"crypto/ecdsa"
	"encoding/hex"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/tinyverse-web3/mtv_go_utils/key"
)

func getKeyBySeed(seed string) (*ecdsa.PrivateKey, *ecdsa.PublicKey, error) {
	prikey, pubkey, err := key.GenerateEcdsaKey(seed)
	if err != nil {
		return nil, nil, err
	}
	return prikey, pubkey, nil
}

func getSeedKey(seed string) (prikey *ecdsa.PrivateKey, pubkey *ecdsa.PublicKey) {
	var err error
	prikey, pubkey, err = getKeyBySeed(seed)
	if err != nil {
		logger.Fatalf("tvnode->main: getKeyBySeed error: %v", err)
	}
	srcPrikeyHex := hex.EncodeToString(crypto.FromECDSA(prikey))
	srcPubkeyHex := hex.EncodeToString(crypto.FromECDSAPub(pubkey))
	logger.Infof("tvnode->main:\nseed: %s\nprikey: %s\npubkey: %s", seed, srcPrikeyHex, srcPubkeyHex)

	return prikey, pubkey
}

func getPubkey(pubkey *ecdsa.PublicKey) (string, error) {
	pubkeyData, err := key.ECDSAPublicKeyToProtoBuf(pubkey)
	if err != nil {
		return "", err
	}
	return key.TranslateKeyProtoBufToString(pubkeyData), nil
}
