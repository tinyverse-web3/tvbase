package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	filelock "github.com/MichaelS11/go-file-lock"
	"github.com/mitchellh/go-homedir"
	tvbaseIpfs "github.com/tinyverse-web3/tvbase/common/ipfs"
	tvbaseUtil "github.com/tinyverse-web3/tvbase/common/util"
	"github.com/tinyverse-web3/tvbase/corehttp"
	syncfile "github.com/tinyverse-web3/tvbase/dmsg/protocol/custom/ipfs"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/custom/pullcid"
	"github.com/tinyverse-web3/tvbase/tvbase"
)

const (
	defaultPathName = ".tvnode"
	defaultPathRoot = "~/" + defaultPathName
)

func getPidFileName(rootPath string) (string, error) {
	rootPath = strings.Trim(rootPath, " ")
	if rootPath == "" {
		rootPath = "."
	}
	fullPath, err := homedir.Expand(rootPath)
	if err != nil {
		fmt.Println("GenConfig2IdentityFile->homedir.Expand: " + err.Error())
		return "", err
	}
	if !filepath.IsAbs(fullPath) {
		defaultRootPath, err := os.Getwd()
		if err != nil {
			fmt.Println("GenConfig2IdentityFile->Getwd: " + err.Error())
			return "", err
		}
		fullPath = filepath.Join(defaultRootPath, fullPath)
	}

	if !strings.HasSuffix(fullPath, string(filepath.Separator)) {
		fullPath += string(filepath.Separator)
	}
	_, err = os.Stat(fullPath)
	if os.IsNotExist(err) {
		err := os.MkdirAll(fullPath, 0755)
		if err != nil {
			fmt.Println("GenConfig2IdentityFile->MkdirAll: " + err.Error())
			return "", err
		}
	}

	pidFile := fullPath + "tvnode.pid"
	return pidFile, nil
}

func main() {
	rootPath := parseCmdParams()
	rootPath, err := tvbaseUtil.GetRootPath(rootPath)
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

	switch *env {
	case localnetTestEnv:
		cfg.Tvbase.SetLocalNet(true)
		cfg.Tvbase.SetDhtProtocolPrefix("/tvnode_test")
		// cfg.DMsg.Pubsub.TraceFile = "pubsub-trace.json"
		cfg.Tvbase.ClearBootstrapPeers()
		cfg.Tvbase.AddBootstrapPeer("/ip4/192.168.1.102/tcp/9000/p2p/12D3KooWGUjKn8SHYjdGsnzjFDT3G33svXCbLYXebsT9vsK8dyHu")
		cfg.Tvbase.AddBootstrapPeer("/ip4/192.168.1.109/tcp/9000/p2p/12D3KooWGhqQa67QMRFAisZSZ1snfCnpFtWtr4rXTZ2iPBfVu1RR")
	case internetTestEnv:
		cfg.Tvbase.SetLocalNet(false)
		cfg.Tvbase.SetDhtProtocolPrefix("/tvnode_test")
		// cfg.DMsg.Pubsub.TraceFile = "pubsub-trace.json"
		cfg.Tvbase.ClearBootstrapPeers()
		cfg.Tvbase.AddBootstrapPeer("/ip4/39.108.147.241/tcp/9000/p2p/12D3KooWJ9BvdU8q6gcEDpDUF42qV3PLaAd8vgh7HGveuktFMHoq")
		cfg.Tvbase.AddBootstrapPeer("/ip4/39.108.96.46/tcp/9000/p2p/12D3KooWDzny9ZpW44Eb2YL5uQQ1CCcgQSQcc851oiB6XyXHG7TM")
	}

	logger.Infof("tvnode->main: BootstrapPeers: %v", cfg.Tvbase.Bootstrap.BootstrapPeers)

	ctx := context.Background()
	tb, err := tvbase.NewTvbase(ctx, cfg.Tvbase, rootPath)
	if err != nil {
		logger.Fatalf("tvnode->main: NewInfrasture :%v", err)
	}

	_, err = tvbaseIpfs.CreateIpfsShellProxy(cfg.CustomProtocol.IpfsSyncFile.IpfsURL)
	if err != nil {
		logger.Errorf("tvnode->main: CreateIpfsShell: %v", err)
		return
	}

	pp, err := pullcid.NewPullCidService()
	if err != nil {
		logger.Fatalf("tvnode->main: GetPullCidServiceProtocol :%v", err)
	}
	tb.RegistCSSProtocol(pp)

	fp, err := syncfile.NewSyncFileUploadService()
	if err != nil {
		logger.Fatalf("tvnode->main: GetFileSyncServiceProtocol :%v", err)
	}
	tb.RegistCSSProtocol(fp)

	cp, err := syncfile.NewSyncFileSummaryService()
	if err != nil {
		logger.Fatalf("tvnode->main: GetSummaryServiceProtocol :%v", err)
	}
	tb.RegistCSSProtocol(cp)

	err = tb.Start()
	if err != nil {
		logger.Fatalf("tvnode->main: Start: %v", err)
	}

	err = tb.DmsgService.Start()
	if err != nil {
		logger.Fatalf("tvnode->main: Start: %v", err)
	}

	corehttp.StartWebService(tb)
	<-ctx.Done()
	// tvInfrasture.Stop()
	// Logger.Info("tvnode_->main: Gracefully shut down daemon")
}
