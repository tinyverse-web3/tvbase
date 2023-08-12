package main

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	filelock "github.com/MichaelS11/go-file-lock"
	"github.com/ethereum/go-ethereum/crypto"
	ipfsLog "github.com/ipfs/go-log/v2"
	"github.com/mitchellh/go-homedir"
	"github.com/tinyverse-web3/tvbase/common/define"
	tvUtil "github.com/tinyverse-web3/tvbase/common/util"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/custom/pullcid"
	dmsgService "github.com/tinyverse-web3/tvbase/dmsg/service"
	"github.com/tinyverse-web3/tvbase/tvbase"
	tvutilCrypto "github.com/tinyverse-web3/tvutil/crypto"
	tvUtilKey "github.com/tinyverse-web3/tvutil/key"
)

const (
	defaultPathName = ".tvnode"
	defaultPathRoot = "~/" + defaultPathName
)

const logName = "tvnode"

var mainLog = ipfsLog.Logger(logName)

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
		mainLog.Info("tinverse tvnode")
		mainLog.Info("Usage step1: Run './tvnode -init' generate identity key and config.")
		mainLog.Info("Usage step2: Run './tvnode' or './tvnode -rootPath .' start tinyverse tvnode service.")
		os.Exit(0)
	}
	if *generateCfg {
		err := tvUtil.GenConfig2IdentityFile(*rootPath, define.ServiceMode)
		if err != nil {
			mainLog.Fatalf("Failed to generate config file: %v", err)
		}
		mainLog.Infof("Generate config file successfully.")
		os.Exit(0)
	}

	if *shutDown {
		pidFile, err := getPidFileName(*rootPath)
		if err != nil {
			mainLog.Infof("Failed to get pidFileName: %v", err)
			os.Exit(0)
		}
		file, err := os.Open(pidFile)
		if err != nil {
			mainLog.Infof("Failed to open pidFile: %v", err)
			os.Exit(0)
		}
		defer file.Close()
		content, err := io.ReadAll(file)
		if err != nil {
			mainLog.Infof("Failed to read pidFile: %v", err)
			os.Exit(0)
		}
		pid, err := strconv.Atoi(strings.TrimRight(string(content), "\r\n"))
		if err != nil {
			mainLog.Errorf("The pidFile content is not a number, content: %v ,error: %v", content, err)
		}

		process, err := os.FindProcess(pid)
		if err != nil {
			mainLog.Infof("Failed to find process: %v", err)
			os.Exit(0)
		}

		err = process.Signal(syscall.SIGKILL)
		if err != nil {
			mainLog.Infof("Failed to terminate process: %v", err)
		}

		mainLog.Infof("Process terminated successfully")
		os.Exit(0)
	}
	return *rootPath
}

func main() {
	rootPath := parseCmdParams()

	nodeConfig, err := tvUtil.LoadNodeConfig(rootPath)
	if err != nil {
		mainLog.Errorf("tvnode->main: %v", err)
		return
	}

	err = tvUtil.SetLogModule(nodeConfig.Log.ModuleLevels)
	if err != nil {
		mainLog.Fatalf("tvnode->main: init log: %v", err)
	}

	pidFileName, err := getPidFileName(rootPath)
	if err != nil {
		mainLog.Fatalf("tvnode->main: get pid file name: %v", err)
	}
	pidFileLockHandle, err := filelock.New(pidFileName)
	mainLog.Infof("tvnode->main: PID: %v", os.Getpid())
	if err == filelock.ErrFileIsBeingUsed {
		mainLog.Errorf("tvnode->main: pid file is being locked: %v", err)
		return
	}
	if err != nil {
		mainLog.Errorf("tvnode->main: pid file lock: %v", err)
		return
	}
	defer func() {
		err = pidFileLockHandle.Unlock()
		if err != nil {
			mainLog.Errorf("tvnode->main: pid file unlock: %v", err)
		}
		err = os.Remove(pidFileName)
		if err != nil {
			mainLog.Errorf("tvnode->main: pid file remove: %v", err)
		}
	}()

	ctx := context.Background()

	//src
	userSeed := "softwarecheng@gmail.com"
	srcPrikey, srcPubkey, err := getKeyBySeed(userSeed)
	if err != nil {
		mainLog.Errorf("tvnode->main: getKeyBySeed error: %v", err)
		return
	}
	srcPrikeyHex := hex.EncodeToString(crypto.FromECDSA(srcPrikey))
	srcPubkeyHex := hex.EncodeToString(crypto.FromECDSAPub(srcPubkey))
	mainLog.Infof("tvnode->main:\nuserSeed: %s\nprikey: %s\npubkey: %s", userSeed, srcPrikeyHex, srcPubkeyHex)

	tvbase, _, err := initDmsg(srcPubkey, srcPrikey, rootPath, ctx)
	if err != nil {
		mainLog.Errorf("tvnode->main: initDmsg: %v", err)
		return
	}

	p, err := pullcid.GetPullCidServiceProtocol(tvbase)
	if err != nil {
		mainLog.Fatalf("tvnode->main: GetPullCidServiceProtocol :%v", err)
	}
	tvbase.RegistCSSProtocol(p)

	<-ctx.Done()
}

func initDmsg(
	srcPubkey *ecdsa.PublicKey,
	srcPrikey *ecdsa.PrivateKey,
	rootPath string,
	ctx context.Context) (*tvbase.TvBase, *dmsgService.DmsgService, error) {
	tvbase, err := tvbase.NewTvbase(rootPath, ctx, true)
	if err != nil {
		mainLog.Fatalf("initDmsg error: %v", err)
	}

	dmsgService := tvbase.GetServiceDmsgService()
	userPubkeyData, err := tvUtilKey.ECDSAPublicKeyToProtoBuf(srcPubkey)
	if err != nil {
		mainLog.Errorf("initDmsg: ECDSAPublicKeyToProtoBuf error: %v", err)
		return nil, nil, err
	}

	getSig := func(protoData []byte) ([]byte, error) {
		sig, err := tvutilCrypto.SignDataByEcdsa(srcPrikey, protoData)
		if err != nil {
			mainLog.Errorf("initDmsg: sig error: %v", err)
		}
		return sig, nil
	}

	err = dmsgService.InitUser(userPubkeyData, getSig)
	if err != nil {
		return nil, nil, err
	}
	return tvbase, dmsgService, nil
}

func getKeyBySeed(seed string) (*ecdsa.PrivateKey, *ecdsa.PublicKey, error) {
	prikey, pubkey, err := tvUtilKey.GenerateEcdsaKey(seed)
	if err != nil {
		return nil, nil, err
	}
	return prikey, pubkey, nil
}
