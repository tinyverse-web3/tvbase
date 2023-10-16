package main

import (
	"context"
	"crypto/ecdsa"
	"os"
	"time"

	"github.com/tinyverse-web3/mtv_go_utils/crypto"
	"github.com/tinyverse-web3/mtv_go_utils/key"
	"github.com/tinyverse-web3/tvbase/common/util"
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
	defer func() {
		err = tb.Stop()
		if err != nil {
			logger.Errorf("tvnode->main: tb.Stop: %v", err)
		}
	}()

	srcPrikey, srcPubkey := getSeedKey("softwarecheng@gmail.com")
	err = startDmsgService(srcPubkey, srcPrikey, tb)
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

func startDmsgService(srcPubkey *ecdsa.PublicKey, srcPrikey *ecdsa.PrivateKey, tb *tvbase.TvBase) error {
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

	dmsgService, err = CreateDmsgService(tb)
	if err != nil {
		return err
	}

	err = dmsgService.Start(true, userPubkeyData, getSig, 30*time.Second)
	if err != nil {
		return err
	}

	dmsgService.GetMailboxService().SetUserPubkey(userPubkeyData, getSig)
	err = dmsgService.GetMailboxService().CreateMailbox(30 * time.Second)
	if err != nil {
		return err
	}

	dmsgService.GetMailboxService().TickReadMailbox(3*time.Minute, 30*time.Second)

	return nil
}
