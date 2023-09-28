package main

import (
	"crypto/ecdsa"
	"encoding/hex"
	"os"
	"path/filepath"

	filelock "github.com/MichaelS11/go-file-lock"
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

func getPidFileName(rootPath string) string {
	return rootPath + filepath.Base(os.Args[0]) + ".pid"
}

func lockProcess(rootPath string) (lock *filelock.LockHandle, pidFileName string) {
	pidFileName = getPidFileName(rootPath)
	lock, err := filelock.New(pidFileName)
	logger.Infof("tvnode->main: PID: %v", os.Getpid())
	if err == filelock.ErrFileIsBeingUsed {
		logger.Fatalf("tvnode->main: pid file is being locked: %v", err)
	}
	if err != nil {
		logger.Fatalf("tvnode->main: pid file lock error: %v", err)
	}
	return lock, pidFileName
}

func getSeedKey(seed string) (prikey *ecdsa.PrivateKey, pubkey *ecdsa.PublicKey) {
	var err error
	prikey, pubkey, err = getKeyBySeed(seed)
	if err != nil {
		logger.Fatalf("tvnode->main: getKeyBySeed error: %v", err)
	}
	srcPrikeyHex := hex.EncodeToString(crypto.FromECDSA(prikey))
	srcPubkeyHex := hex.EncodeToString(crypto.FromECDSAPub(pubkey))
	logger.Infof("tvnode->main:\nuserSeed: %s\nprikey: %s\npubkey: %s", seed, srcPrikeyHex, srcPubkeyHex)

	return prikey, pubkey
}
