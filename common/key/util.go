package key

import (
	"crypto/ecdsa"
	"encoding/hex"

	"github.com/ethereum/go-ethereum/crypto"
)

func PubkeyFromEcdsaHex(pubkeyHex string) (*ecdsa.PublicKey, error) {
	var publicKeyByte []byte
	var err error
	publicKeyByte, err = hex.DecodeString(pubkeyHex)
	if err != nil {
		return nil, err
	}
	ret, err := crypto.UnmarshalPubkey(publicKeyByte)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func PriKeyFromEcdsaHex(prikeyHex string) (*ecdsa.PrivateKey, error) {
	return crypto.HexToECDSA(prikeyHex)
}
