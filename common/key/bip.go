package key

import (
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/tyler-smith/go-bip39"
)

func generateSeed(passphrase string) []byte {
	// Generate a mnemonic for memorization or user-friendly seeds
	entropy, _ := bip39.NewEntropy(256)
	mnemonic, _ := bip39.NewMnemonic(entropy)

	fmt.Println("Mnemonic: ", mnemonic)
	return bip39.NewSeed(mnemonic, passphrase)
}

func newKs(keydir string) *keystore.KeyStore {
	return keystore.NewKeyStore(keydir, keystore.StandardScryptN, keystore.StandardScryptP)
}
