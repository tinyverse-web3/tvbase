package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	filelock "github.com/MichaelS11/go-file-lock"
	ipfsLog "github.com/ipfs/go-log/v2"
	"github.com/mitchellh/go-homedir"
	"github.com/tinyverse-web3/tvbase/common/config"
	tvbaseIpfs "github.com/tinyverse-web3/tvbase/common/ipfs"
	"github.com/tinyverse-web3/tvbase/common/load"
	tvbaseUtil "github.com/tinyverse-web3/tvbase/common/util"
	syncfile "github.com/tinyverse-web3/tvbase/dmsg/protocol/custom/ipfs"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/custom/pullcid"
	"github.com/tinyverse-web3/tvbase/tvbase"
)

const (
	defaultPathName = ".tvnode"
	defaultPathRoot = "~/" + defaultPathName
)

const logName = "tvnode"

var tvsLog = ipfsLog.Logger(logName)

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

func parseCmdParams() string {
	generateCfg := flag.Bool("init", false, "init generate identity key and config file")
	rootPath := flag.String("rootPath", defaultPathRoot, "config file path")
	shutDown := flag.Bool("shutdown", false, "shutdown daemon")
	help := flag.Bool("help", false, "Display help")

	flag.Parse()

	if *help {
		tvsLog.Info("tinverse tvnode")
		tvsLog.Info("Usage step1: Run './tvnode -init' generate identity key and config.")
		tvsLog.Info("Usage step2: Run './tvnode' or './tvnode -rootPath .' start tinyverse tvnode service.")
		os.Exit(0)
	}
	if *generateCfg {
		fullPath, err := tvbaseUtil.GetRootPath(*rootPath)
		if err != nil {
			tvsLog.Fatalf("GetRootPath error: %v", err)
		}
		_, err = os.Stat(fullPath)
		if os.IsNotExist(err) {
			err := os.MkdirAll(fullPath, 0755)
			if err != nil {
				tvsLog.Fatalf("MkdirAll error: %v", err)
			}
		}
		err = load.GenConfigFile(fullPath, config.ServiceMode)
		if err != nil {
			tvsLog.Fatalf("Failed to generate config file: %v", err)
		}
		err = load.GenIdentityFile(fullPath)
		if err != nil {
			tvsLog.Fatalf("Failed to generate config file: %v", err)
		}
		tvsLog.Infof("Generate config file successfully.")
		os.Exit(0)
	}

	if *shutDown {
		pidFile, err := getPidFileName(*rootPath)
		if err != nil {
			tvsLog.Infof("Failed to get pidFileName: %v", err)
			os.Exit(0)
		}
		file, err := os.Open(pidFile)
		if err != nil {
			tvsLog.Infof("Failed to open pidFile: %v", err)
			os.Exit(0)
		}
		defer file.Close()
		content, err := io.ReadAll(file)
		if err != nil {
			tvsLog.Infof("Failed to read pidFile: %v", err)
			os.Exit(0)
		}
		pid, err := strconv.Atoi(strings.TrimRight(string(content), "\r\n"))
		if err != nil {
			tvsLog.Errorf("The pidFile content is not a number, content: %v ,error: %v", content, err)
		}

		process, err := os.FindProcess(pid)
		if err != nil {
			tvsLog.Infof("Failed to find process: %v", err)
			os.Exit(0)
		}

		err = process.Signal(syscall.SIGKILL)
		if err != nil {
			tvsLog.Infof("Failed to terminate process: %v", err)
		}

		tvsLog.Infof("Process terminated successfully")
		os.Exit(0)
	}
	return *rootPath
}

func initLog(cfg *config.LogConfig) (err error) {
	err = tvbaseUtil.SetLogModule(cfg.ModuleLevels)
	if err != nil {
		return err
	}
	return nil
}

func main() {
	rootPath := parseCmdParams()
	rootPath, err := tvbaseUtil.GetRootPath(rootPath)
	if err != nil {
		tvsLog.Fatalf("tvnode->main: GetRootPath: %v", err)
	}
	pidFileName, err := getPidFileName(rootPath)
	if err != nil {
		tvsLog.Fatalf("tvnode->main: get pid file name: %v", err)
	}
	pidFileLockHandle, err := filelock.New(pidFileName)
	tvsLog.Infof("tvnode->main: PID: %v", os.Getpid())
	if err == filelock.ErrFileIsBeingUsed {
		tvsLog.Errorf("tvnode->main: pid file is being locked: %v", err)
		return
	}
	if err != nil {
		tvsLog.Errorf("tvnode->main: pid file lock: %v", err)
		return
	}
	defer func() {
		err = pidFileLockHandle.Unlock()
		if err != nil {
			tvsLog.Errorf("tvnode->main: pid file unlock: %v", err)
		}
		err = os.Remove(pidFileName)
		if err != nil {
			tvsLog.Errorf("tvnode->main: pid file remove: %v", err)
		}
	}()

	cfg, err := load.LoadConfig(rootPath)
	if err != nil || cfg == nil {
		tvsLog.Fatalf("tvnode->main: loadConfig: %v", err)
	}

	err = initLog(cfg.Log)
	if err != nil {
		tvsLog.Fatalf("tvnode->main: initLog: %v", err)
	}

	// test enviroment
	// cfg.Tvbase.SetLocalNet(true)
	// cfg.Tvbase.SetMdns(false)
	// cfg.Tvbase.SetDhtProtocolPrefix("/tvnode_test")
	// cfg.Tvbase.ClearBootstrapPeers()
	// cfg.Tvbase.AddBootstrapPeer("/ip4/192.168.1.102/tcp/9000/p2p/12D3KooWPThTtBAaC5vvnj6NE2iQSfuBHRUdtPweM6dER62R57R2")
	// cfg.Tvbase.AddBootstrapPeer("/ip4/192.168.1.109/tcp/9000/p2p/12D3KooWQvMGQWCRGdjtaFvqbdQ7qf8cw1x94hy1mWMvQovF6uAE")

	ctx := context.Background()
	tb, err := tvbase.NewTvbase(ctx, cfg.Tvbase, rootPath)
	if err != nil {
		tvsLog.Fatalf("tvnode->main: NewInfrasture :%v", err)
	}

	_, err = tvbaseIpfs.CreateIpfsShellProxy(cfg.CustomProtocol.IpfsSyncFile.IpfsURL)
	if err != nil {
		tvsLog.Errorf("tvnode->main: CreateIpfsShell: %v", err)
		return
	}

	pp, err := pullcid.NewPullCidService()
	if err != nil {
		tvsLog.Fatalf("tvnode->main: GetPullCidServiceProtocol :%v", err)
	}
	tb.RegistCSSProtocol(pp)

	fp, err := syncfile.NewSyncFileUploadService()
	if err != nil {
		tvsLog.Fatalf("tvnode->main: GetFileSyncServiceProtocol :%v", err)
	}
	tb.RegistCSSProtocol(fp)

	cp, err := syncfile.NewSyncFileSummaryService()
	if err != nil {
		tvsLog.Fatalf("tvnode->main: GetSummaryServiceProtocol :%v", err)
	}
	tb.RegistCSSProtocol(cp)

	tb.Start()
	<-ctx.Done()
	// tvInfrasture.Stop()
	// Logger.Info("tvnode_->main: Gracefully shut down daemon")
}
