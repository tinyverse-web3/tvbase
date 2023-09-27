package main

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	filelock "github.com/MichaelS11/go-file-lock"
	"github.com/ethereum/go-ethereum/crypto"
	ipfsLog "github.com/ipfs/go-log/v2"
	"github.com/mitchellh/go-homedir"
	tvutilCrypto "github.com/tinyverse-web3/mtv_go_utils/crypto"
	ipfsUtil "github.com/tinyverse-web3/mtv_go_utils/ipfs"
	tvUtilKey "github.com/tinyverse-web3/mtv_go_utils/key"
	"github.com/tinyverse-web3/tvbase/common/config"
	"github.com/tinyverse-web3/tvbase/common/identity"
	tvbaseUtil "github.com/tinyverse-web3/tvbase/common/util"
	"github.com/tinyverse-web3/tvbase/dmsg/service"
	"github.com/tinyverse-web3/tvbase/tvbase"
)

const (
	defaultPathName = ".tvnode"
	defaultPathRoot = "~/" + defaultPathName
	configFileName  = "config.json"
	logName         = "tvnode"
)

var mainLog = ipfsLog.Logger(logName)

func main() {
	rootPath := parseCmdParams()
	rootPath, err := tvbaseUtil.GetRootPath(rootPath)
	if err != nil {
		mainLog.Fatalf("tvnode->main: GetRootPath: %v", err)
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

	cfg, err := loadConfig(rootPath)
	if err != nil || cfg == nil {
		mainLog.Fatalf("tvnode->main: loadConfig: %v", err)
	}

	err = initLog()
	if err != nil {
		mainLog.Fatalf("tvnode->main: initLog: %v", err)
	}

	setTestEnv(cfg)

	ctx := context.Background()
	userSeed := "softwarecheng@gmail.com"
	srcPrikey, srcPubkey, err := getKeyBySeed(userSeed)
	if err != nil {
		mainLog.Errorf("tvnode->main: getKeyBySeed error: %v", err)
		return
	}
	srcPrikeyHex := hex.EncodeToString(crypto.FromECDSA(srcPrikey))
	srcPubkeyHex := hex.EncodeToString(crypto.FromECDSAPub(srcPubkey))
	mainLog.Infof("tvnode->main:\nuserSeed: %s\nprikey: %s\npubkey: %s", userSeed, srcPrikeyHex, srcPubkeyHex)

	tb, _, err := initService(srcPubkey, srcPrikey, cfg, rootPath, ctx)
	if err != nil {
		mainLog.Errorf("tvnode->main: initDmsg: %v", err)
		return
	}

	_, err = ipfsUtil.CreateIpfsShellProxy("/ip4/127.0.0.1/tcp/5001")
	if err != nil {
		mainLog.Errorf("tvnode->main: CreateIpfsShell: %v", err)
		return
	}

	tb.Start()
	<-ctx.Done()
}

func setTestEnv(cfg *config.TvbaseConfig) {
	// test enviroment
	cfg.SetLocalNet(true)
	cfg.SetMdns(false)
	cfg.SetDhtProtocolPrefix("/tvnode_test")
	cfg.ClearBootstrapPeers()
	cfg.AddBootstrapPeer("/ip4/192.168.1.102/tcp/9000/p2p/12D3KooWPThTtBAaC5vvnj6NE2iQSfuBHRUdtPweM6dER62R57R2")
	cfg.AddBootstrapPeer("/ip4/192.168.1.109/tcp/9000/p2p/12D3KooWQvMGQWCRGdjtaFvqbdQ7qf8cw1x94hy1mWMvQovF6uAE")
}

func initService(
	srcPubkey *ecdsa.PublicKey,
	srcPrikey *ecdsa.PrivateKey,
	cfg *config.TvbaseConfig,
	rootPath string,
	ctx context.Context) (*tvbase.TvBase, *service.DmsgService, error) {
	tb, err := tvbase.NewTvbase(ctx, cfg, rootPath)
	if err != nil {
		mainLog.Fatalf("initDmsg error: %v", err)
	}

	dmsgService := tb.GetDmsgService()
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

	err = dmsgService.Start(true, userPubkeyData, getSig, 3*time.Second)
	if err != nil {
		return nil, nil, err
	}
	return tb, dmsgService, nil
}

func getKeyBySeed(seed string) (*ecdsa.PrivateKey, *ecdsa.PublicKey, error) {
	prikey, pubkey, err := tvUtilKey.GenerateEcdsaKey(seed)
	if err != nil {
		return nil, nil, err
	}
	return prikey, pubkey, nil
}

func loadConfig(rootPath string) (*config.TvbaseConfig, error) {
	ret := &config.TvbaseConfig{}

	configFilePath := rootPath + configFileName
	_, err := os.Stat(configFilePath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	err = config.LoadConfig(ret, configFilePath)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func genConfigFile(rootPath string, mode config.NodeMode) error {
	defaultCfg := config.NewDefaultTvbaseConfig()
	cfg, err := loadConfig(rootPath)
	if err != nil {
		return err
	}
	if cfg == nil {
		cfg = defaultCfg
	} else {
		if err != nil {
			return err
		}
	}
	cfg.InitMode(mode)
	file, _ := json.MarshalIndent(cfg, "", " ")
	if err := os.WriteFile(rootPath+configFileName, file, 0644); err != nil {
		fmt.Println("CreateConfigFileIfNotExist: Failed to WriteFile:", err)
		return err
	}
	fmt.Println("genConfigFile->generate node config file: " + rootPath + configFileName)
	return nil
}

func genIdentityFile(rootPath string) error {
	err := identity.GenIdenityFile(rootPath)
	if err != nil {
		fmt.Println("GenConfig2IdentityFile->GenIdenityFile: " + err.Error())
	}
	fmt.Println("GenConfig2IdentityFile->generate identity file: " + rootPath + identity.IdentityFileName)
	return nil
}

func getPidFileName(rootPath string) (string, error) {
	rootPath = strings.Trim(rootPath, " ")
	if rootPath == "" {
		rootPath = "."
	}
	fullPath, err := homedir.Expand(rootPath)
	if err != nil {
		fmt.Println("getPidFileName->homedir.Expand: " + err.Error())
		return "", err
	}
	if !filepath.IsAbs(fullPath) {
		defaultRootPath, err := os.Getwd()
		if err != nil {
			fmt.Println("getPidFileName->Getwd: " + err.Error())
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
			fmt.Println("getPidFileName->MkdirAll: " + err.Error())
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
		fullPath, err := tvbaseUtil.GetRootPath(*rootPath)
		if err != nil {
			mainLog.Fatalf("GetRootPath error: %v", err)
		}
		_, err = os.Stat(fullPath)
		if os.IsNotExist(err) {
			err := os.MkdirAll(fullPath, 0755)
			if err != nil {
				mainLog.Fatalf("MkdirAll error: %v", err)
			}
		}
		err = genConfigFile(fullPath, config.ServiceMode)
		if err != nil {
			mainLog.Fatalf("Failed to generate config file: %v", err)
		}
		err = genIdentityFile(fullPath)
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

func initLog() (err error) {
	var moduleLevels = map[string]string{
		"tvbase":         "debug",
		"dkvs":           "debug",
		"dmsg":           "debug",
		"customProtocol": "debug",
		"tvnode":         "debug",
		"tvipfs":         "debug",
		"core_http":      "debug",
	}
	err = tvbaseUtil.SetLogModule(moduleLevels)
	if err != nil {
		return err
	}
	return nil
}
