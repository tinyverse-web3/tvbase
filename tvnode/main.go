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
	tvConfig "github.com/tinyverse-web3/tvbase/common/config"
	tvUtil "github.com/tinyverse-web3/tvbase/common/util"
	ipfsCustomProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol/custom/ipfs"
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
		err := tvUtil.GenConfig2IdentityFile(*rootPath, tvConfig.ServiceMode)
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

func main() {
	rootPath := parseCmdParams()

	nodeConfig, err := tvUtil.LoadNodeConfig(rootPath)
	if err != nil {
		tvsLog.Errorf("tvnode->main: %v", err)
		return
	}

	err = tvUtil.SetLogModule(nodeConfig.Log.ModuleLevels)
	if err != nil {
		tvsLog.Fatalf("tvnode->main: init log: %v", err)
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

	ctx := context.Background()
	tb, err := tvbase.NewTvbase(rootPath, ctx, true)
	if err != nil {
		tvsLog.Fatalf("tvnode->main: NewInfrasture :%v", err)
	}
	p, err := ipfsCustomProtocol.GetServiceProtocol(tb)
	if err != nil {
		tvsLog.Fatalf("tvnode->main: GetPullCidServiceProtocol :%v", err)
	}
	tb.RegistCSSProtocol(p)

	<-ctx.Done()
	// tvInfrasture.Stop()
	// Logger.Info("tvnode_->main: Gracefully shut down daemon")
}
