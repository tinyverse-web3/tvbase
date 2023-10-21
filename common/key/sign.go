package key

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
)

func Sign(privateKey *ecdsa.PrivateKey, message []byte) ([]byte, error) {
	hash := sha256.Sum256(message)
	signature, err := ecdsa.SignASN1(rand.Reader, privateKey, hash[:])
	return signature, err
}

func Verify(publicKey *ecdsa.PublicKey, data, sig []byte) (bool, error) {
	hash := sha256.Sum256(data)
	return ecdsa.VerifyASN1(publicKey, hash[:], sig), nil
}
