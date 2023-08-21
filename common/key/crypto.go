package key

import (
	"crypto/ecdsa"
	"crypto/rand"

	"github.com/ethereum/go-ethereum/crypto/ecies"
)

func EncryptWithPubkey(publicKey *ecdsa.PublicKey, content []byte) ([]byte, error) {
	eciesPublicKey := ecies.PublicKey{
		Curve: publicKey.Curve,
		X:     publicKey.X,
		Y:     publicKey.Y,
	}
	ciphertext, err := ecies.Encrypt(rand.Reader, &eciesPublicKey, content, nil, nil)
	if err != nil {
		return nil, err
	}
	return ciphertext, nil
}

func DecryptWithPrikey(prikey *ecdsa.PrivateKey, ciphertContent []byte) ([]byte, error) {
	eciesPrivateKey := ecies.ImportECDSA(prikey)
	decrypted, err := eciesPrivateKey.Decrypt(ciphertContent, nil, nil)
	if err != nil {
		return nil, err
	}
	return decrypted, nil
}
