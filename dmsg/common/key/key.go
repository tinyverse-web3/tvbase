package key

import (
	"crypto/ecdsa"
	// "fmt"

	"github.com/tinyverse-web3/tvbase/dmsg/common/log"
	"github.com/tinyverse-web3/tvutil/crypto"
	"github.com/tinyverse-web3/tvutil/key"
)

// key
func NewKey() *Key {
	return &Key{}
}

func (k *Key) InitKeyWithPrikey(prikey *ecdsa.PrivateKey) error {
	pubkey := &prikey.PublicKey
	prikeyData, err := key.ECDSAPrivateKeyToProtoBuf(prikey)
	if err != nil {
		log.Logger.Errorf("Key->InitKeyWithPrikey: ECDSAPrivateKeyToProtoBuf error: %v", err)
		return err
	}
	prikeyHex := key.TranslateKeyProtoBufToString(prikeyData)
	pubkeyData, err := key.ECDSAPublicKeyToProtoBuf(pubkey)
	if err != nil {
		log.Logger.Errorf("Key->InitKeyWithPrikey: ECDSAPublicKeyToProtoBuf error: %v", err)
		return err
	}
	pubkeyHex := key.TranslateKeyProtoBufToString(pubkeyData)

	getSigCallback := func(data []byte) (sig []byte, err error) {
		return crypto.SignDataByEcdsa(prikey, data)
	}
	k = &Key{
		PubkeyHex: pubkeyHex,
		Pubkey:    pubkey,
		PrikeyHex: prikeyHex,
		Prikey:    prikey,
		GetSig:    getSigCallback,
	}
	return nil
}

func (k *Key) InitKeyWithPubkeyData(pubkeyData []byte, getSig GetSigCallback) error {
	pubkeyHex := key.TranslateKeyProtoBufToString(pubkeyData)
	pubkey, err := key.ECDSAProtoBufToPublicKey(pubkeyData)
	if err != nil {
		log.Logger.Errorf("UserManager->AddUserWithPubkeyData: Public key is not ECDSA KEY")
		return err
	}
	k = &Key{
		PubkeyHex: pubkeyHex,
		Pubkey:    pubkey,
		PrikeyHex: "",
		Prikey:    nil,
		GetSig:    getSig,
	}
	return nil
}

func (k *Key) InitKeyWithPubkeyHex(pubkeyHex string, getSig GetSigCallback) error {
	pubkeyData, err := key.TranslateKeyStringToProtoBuf(pubkeyHex)
	if err != nil {
		log.Logger.Errorf("Key->InitKeyWithPubkeyHex: TranslateKeyStringToProtoBuf error: %v", err)
		return err
	}
	pubkey, err := key.ECDSAProtoBufToPublicKey(pubkeyData)
	if err != nil {
		log.Logger.Errorf("Key->InitKeyWithPubkeyHex: Public key is not ECDSA KEY")
		return err
	}
	k = &Key{
		PubkeyHex: pubkeyHex,
		Pubkey:    pubkey,
		PrikeyHex: "",
		Prikey:    nil,
		GetSig:    getSig,
	}
	return nil
}
