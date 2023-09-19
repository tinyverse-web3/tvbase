package key

import (
	"crypto/ecdsa"
	// "fmt"

	"github.com/tinyverse-web3/mtv_go_utils/crypto"
	"github.com/tinyverse-web3/mtv_go_utils/key"
	"github.com/tinyverse-web3/tvbase/dmsg/common/log"
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

	getSig := func(data []byte) (sig []byte, err error) {
		return crypto.SignDataByEcdsa(prikey, data)
	}
	k.PubkeyHex = pubkeyHex
	k.Pubkey = pubkey
	k.PrikeyHex = prikeyHex
	k.Prikey = prikey
	k.GetSig = getSig
	return nil
}

func (k *Key) InitKeyWithPubkeyData(pubkeyData []byte, getSig GetSigCallback) error {
	pubkeyHex := key.TranslateKeyProtoBufToString(pubkeyData)
	pubkey, err := key.ECDSAProtoBufToPublicKey(pubkeyData)
	if err != nil {
		log.Logger.Errorf("UserManager->AddUserWithPubkeyData: Public key is not ECDSA KEY")
		return err
	}
	k.PubkeyHex = pubkeyHex
	k.Pubkey = pubkey
	k.PrikeyHex = ""
	k.Prikey = nil
	k.GetSig = getSig
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
	k.PubkeyHex = pubkeyHex
	k.Pubkey = pubkey
	k.PrikeyHex = ""
	k.Prikey = nil
	k.GetSig = getSig
	return nil
}
