package key

import "crypto/ecdsa"

type GetSigCallback func(protoData []byte) (sig []byte, err error)

type Key struct {
	PubkeyHex string
	Pubkey    *ecdsa.PublicKey
	PrikeyHex string
	Prikey    *ecdsa.PrivateKey
	GetSig    GetSigCallback
}
