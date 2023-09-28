package main

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"os"
	"time"

	filelock "github.com/MichaelS11/go-file-lock"
	"github.com/ethereum/go-ethereum/crypto"
	tvutilCrypto "github.com/tinyverse-web3/mtv_go_utils/crypto"
	"github.com/tinyverse-web3/mtv_go_utils/ipfs"
	"github.com/tinyverse-web3/mtv_go_utils/key"
	"github.com/tinyverse-web3/tvbase/common/config"
	"github.com/tinyverse-web3/tvbase/common/util"
	"github.com/tinyverse-web3/tvbase/tvbase"
)

func setTestEnv(cfg *config.TvbaseConfig) {
	// test enviroment
	cfg.SetLocalNet(true)
	cfg.SetMdns(false)
	cfg.SetDhtProtocolPrefix("/tvnode_test")
	cfg.ClearBootstrapPeers()
	cfg.AddBootstrapPeer("/ip4/192.168.1.102/tcp/9000/p2p/12D3KooWPThTtBAaC5vvnj6NE2iQSfuBHRUdtPweM6dER62R57R2")
	cfg.AddBootstrapPeer("/ip4/192.168.1.109/tcp/9000/p2p/12D3KooWQvMGQWCRGdjtaFvqbdQ7qf8cw1x94hy1mWMvQovF6uAE")
}

func main() {
	rootPath := parseCmdParams()
	rootPath, err := util.GetRootPath(rootPath)
	if err != nil {
		logger.Fatalf("tvnode->main: GetRootPath: %v", err)
	}
	pidFileName, err := getPidFileName(rootPath)
	if err != nil {
		logger.Fatalf("tvnode->main: get pid file name: %v", err)
	}
	pidFileLockHandle, err := filelock.New(pidFileName)
	logger.Infof("tvnode->main: PID: %v", os.Getpid())
	if err == filelock.ErrFileIsBeingUsed {
		logger.Errorf("tvnode->main: pid file is being locked: %v", err)
		return
	}
	if err != nil {
		logger.Errorf("tvnode->main: pid file lock: %v", err)
		return
	}
	defer func() {
		err = pidFileLockHandle.Unlock()
		if err != nil {
			logger.Errorf("tvnode->main: pid file unlock: %v", err)
		}
		err = os.Remove(pidFileName)
		if err != nil {
			logger.Errorf("tvnode->main: pid file remove: %v", err)
		}
	}()

	cfg, err := loadConfig(rootPath)
	if err != nil || cfg == nil {
		logger.Fatalf("tvnode->main: loadConfig: %v", err)
	}

	err = initLog()
	if err != nil {
		logger.Fatalf("tvnode->main: initLog: %v", err)
	}

	if false {
		setTestEnv(cfg)
	}

	ctx := context.Background()
	userSeed := "softwarecheng@gmail.com"
	srcPrikey, srcPubkey, err := getKeyBySeed(userSeed)
	if err != nil {
		logger.Errorf("tvnode->main: getKeyBySeed error: %v", err)
		return
	}
	srcPrikeyHex := hex.EncodeToString(crypto.FromECDSA(srcPrikey))
	srcPubkeyHex := hex.EncodeToString(crypto.FromECDSAPub(srcPubkey))
	logger.Infof("tvnode->main:\nuserSeed: %s\nprikey: %s\npubkey: %s", userSeed, srcPrikeyHex, srcPubkeyHex)

	tb, err := tvbase.NewTvbase(ctx, cfg, rootPath)
	if err != nil {
		logger.Fatalf("tvnode->main: NewTvbase error: %v", err)
	}
	tb.Start()

	err = startDmsgService(srcPubkey, srcPrikey, tb)
	if err != nil {
		logger.Errorf("tvnode->main: initDmsgService: %v", err)
		return
	}

	_, err = ipfs.CreateIpfsShellProxy("/ip4/127.0.0.1/tcp/5001")
	if err != nil {
		logger.Errorf("tvnode->main: CreateIpfsShell: %v", err)
		return
	}

	<-ctx.Done()
}

func startDmsgService(srcPubkey *ecdsa.PublicKey, srcPrikey *ecdsa.PrivateKey, tb *tvbase.TvBase) error {
	userPubkeyData, err := key.ECDSAPublicKeyToProtoBuf(srcPubkey)
	if err != nil {
		logger.Errorf("initDmsg: ECDSAPublicKeyToProtoBuf error: %v", err)
		return err
	}

	getSig := func(protoData []byte) ([]byte, error) {
		sig, err := tvutilCrypto.SignDataByEcdsa(srcPrikey, protoData)
		if err != nil {
			logger.Errorf("initDmsg: sig error: %v", err)
		}
		return sig, nil
	}

	err = tb.GetDmsgService().Start(true, userPubkeyData, getSig, 30*time.Second)
	if err != nil {
		return err
	}
	return nil
}
