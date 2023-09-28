package identity

import (
	"bytes"
	"os"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/pnet"
	"golang.org/x/crypto/salsa20"
	"golang.org/x/crypto/sha3"
)

const (
	IdentityFileName = "identity.bin"
	SwarmPskFileName = "swarmPsv1key.bin"
)

// LoadIdentity reads a private key from the given path.
func LoadIdentity(path string) (crypto.PrivKey, error) {
	bytes, err := os.ReadFile(path + IdentityFileName)
	if err != nil {
		return nil, err
	}

	return crypto.UnmarshalPrivateKey(bytes)
}

// generateIdentityFile writes a new random private key to the given path.
func generateIdentityFile(path string) (crypto.PrivKey, error) {
	privk, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 0)
	if err != nil {
		return privk, err
	}

	bytes, err := crypto.MarshalPrivateKey(privk)
	if err != nil {
		return privk, err
	}

	err = os.WriteFile(path, bytes, 0400)
	return privk, err
}

// PNet fingerprint section is taken from github.com/ipfs/kubo/core/node/libp2p/pnet.go
// since the functions in that package were not exported.
// https://github.com/ipfs/kubo/blob/255e64e49e837afce534555f3451e2cffe9f0dcb/core/node/libp2p/pnet.go#L74

type PNetFingerprint []byte

// PNetFingerprint returns the given swarm key's fingerprint.
func pnetFingerprint(psk pnet.PSK) []byte {
	var pskArr [32]byte
	copy(pskArr[:], psk)

	enc := make([]byte, 64)
	zeros := make([]byte, 64)
	out := make([]byte, 16)

	// We encrypt data first so we don't feed PSK to hash function.
	// Salsa20 function is not reversible thus increasing our security margin.
	salsa20.XORKeyStream(enc, zeros, []byte("finprint"), &pskArr)

	// Then do Shake-128 hash to reduce its length.
	// This way if for some reason Shake is broken and Salsa20 preimage is possible,
	// attacker has only half of the bytes necessary to recreate psk.
	sha3.ShakeSum128(out, enc)

	return out
}

// LoadSwarmKey loads a swarm key at the given filepath and decodes it as a PSKv1.
func LoadSwarmKey(path string) (pnet.PSK, PNetFingerprint, error) {
	pskBytes, err := os.ReadFile(path)
	if err != nil || pskBytes == nil {
		return nil, nil, err
	}

	psk, err := pnet.DecodeV1PSK(bytes.NewReader(pskBytes))
	if err != nil {
		return nil, nil, err
	}

	return psk, pnetFingerprint(psk), nil
}

func GenIdenityFile(rootPath string) (crypto.PrivKey, error) {
	privateKey, err := generateIdentityFile(rootPath + IdentityFileName)
	if err != nil {
		return nil, err
	}
	return privateKey, nil
}
