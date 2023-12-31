package main

import (
	"context"
	"crypto/ecdsa"
	"os"
	"time"

	"github.com/tinyverse-web3/mtv_go_utils/crypto"
	"github.com/tinyverse-web3/mtv_go_utils/key"
	"github.com/tinyverse-web3/tvbase/common/util"
	"github.com/tinyverse-web3/tvbase/corehttp"
	"github.com/tinyverse-web3/tvbase/tvbase"
)

var tb *tvbase.TvBase
var dmsgService *DmsgService

func main() {
	rootPath := parseCmdParams()
	rootPath, err := util.GetRootPath(rootPath)
	if err != nil {
		logger.Fatalf("tvnode->main: GetRootPath: %v", err)
	}

	lock, pidFileName := lockProcess(rootPath)
	defer func() {
		err = lock.Unlock()
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

	if cfg.Identity.PrivKey == "" {
		err = cfg.GenPrivKey()
		if err != nil {
			logger.Fatalf("tvnode->main: GenPrivKey: %v", err)
		}
	}

	if isTestEnv {
		setTestEnv(cfg)
	}

	err = initLog()
	if err != nil {
		logger.Fatalf("tvnode->main: initLog: %v", err)
	}

	ctx := context.Background()
	tb, err = tvbase.NewTvbase(ctx, cfg, rootPath)
	if err != nil {
		logger.Fatalf("tvnode->main: NewTvbase error: %v", err)
	}

	tb.Start()
	err = tb.WaitRendezvous(30 * time.Second)
	if err != nil {
		logger.Fatalf("tvnode->main: WaitRendezvous error: %v", err)
	}

	corehttp.StartWebService(tb)

	defer func() {
		err = tb.Stop()
		if err != nil {
			logger.Errorf("tvnode->main: tb.Stop: %v", err)
		}
	}()

	srcPrikey, srcPubkey := getSeedKey("softwarecheng@gmail.com")
	err = startDmsgService(srcPubkey, srcPrikey, tb, true)
	if err != nil {
		logger.Errorf("tvnode->main: startDmsgService: %v", err)
		return
	}
	defer func() {
		err = dmsgService.Stop()
		if err != nil {
			logger.Errorf("tvnode->main: tb.Stop: %v", err)
		}
	}()

	<-ctx.Done()
}

func startDmsgService(srcPubkey *ecdsa.PublicKey, srcPrikey *ecdsa.PrivateKey,
	tb *tvbase.TvBase, isListenMsg bool) error {
	userPubkeyData, err := key.ECDSAPublicKeyToProtoBuf(srcPubkey)
	if err != nil {
		logger.Errorf("initDmsg: ECDSAPublicKeyToProtoBuf error: %v", err)
		return err
	}

	getSig := func(protoData []byte) ([]byte, error) {
		sig, err := crypto.SignDataByEcdsa(srcPrikey, protoData)
		if err != nil {
			logger.Errorf("initDmsg: sig error: %v", err)
		}
		return sig, nil
	}

	pubkey := key.TranslateKeyProtoBufToString(userPubkeyData)

	dmsgService, err = CreateDmsgService(tb, pubkey, getSig, isListenMsg)
	if err != nil {
		return err
	}

	err = dmsgService.Start()
	if err != nil {
		return err
	}

	_, err = dmsgService.GetMailboxClient().CreateMailbox(3 * time.Second)
	if err != nil {
		return err
	}

	dmsgService.GetMailboxClient().CreateMailbox(30 * time.Second)
	return nil
}
