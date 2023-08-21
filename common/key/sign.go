package key

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"

	"github.com/ethereum/go-ethereum/crypto"
)

func GenerateEcdsaKey(seedHex string) (*ecdsa.PrivateKey, *ecdsa.PublicKey, error) {
	seedHash := sha256.Sum256([]byte(seedHex))
	seed := make([]byte, ed25519.SeedSize)
	copy(seed, seedHash[0:31])

	cryptoPrikey, err := crypto.ToECDSA(seed)
	if err != nil {
		return nil, nil, err
	}
	return cryptoPrikey, &cryptoPrikey.PublicKey, nil
}

func Sign(privateKey *ecdsa.PrivateKey, message []byte) ([]byte, error) {
	hash := sha256.Sum256(message)
	signature, err := ecdsa.SignASN1(rand.Reader, privateKey, hash[:])
	return signature, err
}

func Verify(publicKey *ecdsa.PublicKey, data, sig []byte) (bool, error) {
	hash := sha256.Sum256(data)
	return ecdsa.VerifyASN1(publicKey, hash[:], sig), nil
}
