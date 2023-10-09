package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/tinyverse-web3/tvbase/common/config"
	"github.com/tinyverse-web3/tvbase/common/identity"
	tvbaseUtil "github.com/tinyverse-web3/tvbase/common/util"
)

const (
	defaultDirName = ".tvnode"
	defaultPath    = "~/" + defaultDirName
)

var isTestEnv = false

func parseCmdParams() string {
	init := flag.Bool("init", false, "Initialize tvnode with default setting configuration file if not already initialized.")
	mode := flag.String("mode", "service", "Initialize tvnode mode for service mode or light mode.")
	path := flag.String("path", defaultPath, "Path to configuration file and data file to use.")
	shutdown := flag.Bool("shutdown", false, "Shutdown the daemon process.")
	help := flag.Bool("help", false, "Show help.")
	showpeer := flag.Bool("showpeer", false, "Show peer ID.")
	test := flag.Bool("test", false, "Run in test environment for different config, example bootstrap node...")
	flag.Parse()

	if *help {
		logger.Info("tinverse tvnode")
		logger.Info("Usage step1: Run './tvnode -init' generate identity key and config.")
		logger.Info("Usage step2: Run './tvnode' or './tvnode -path .' start tinyverse tvnode service.")
		os.Exit(0)
	}

	if *test {
		isTestEnv = *test
	}

	if *showpeer {
		prikey, err := identity.LoadPrikey(tb.GetConfig().Identity.PrivKey)
		if err != nil {
			logger.Fatalf("LoadIdentity error: %v", err)
		}
		printPriKey(prikey)
		os.Exit(0)
	}
	if *init {
		nodeMode := config.ServiceMode
		if *mode == "light" {
			nodeMode = config.LightMode
		}

		dataPath, err := tvbaseUtil.GetRootPath(*path)
		if err != nil {
			logger.Fatalf("GetRootPath error: %v", err)
		}
		_, err = os.Stat(dataPath)
		if os.IsNotExist(err) {
			err := os.MkdirAll(dataPath, 0755)
			if err != nil {
				logger.Fatalf("MkdirAll error: %v", err)
			}
		}
		err = genConfigFile(dataPath, nodeMode)
		if err != nil {
			logger.Fatalf("Failed to generate config file: %v", err)
		}
		os.Exit(0)
	}

	if *shutdown {
		pidFileName := getPidFileName(tb.GetRootPath())
		file, err := os.Open(pidFileName)
		if err != nil {
			logger.Infof("Failed to open pidFile: %v", err)
			os.Exit(0)
		}
		defer file.Close()
		content, err := io.ReadAll(file)
		if err != nil {
			logger.Infof("Failed to read pidFile: %v", err)
			os.Exit(0)
		}
		pid, err := strconv.Atoi(strings.TrimRight(string(content), "\r\n"))
		if err != nil {
			logger.Errorf("The pidFile content is not a number, content: %v ,error: %v", content, err)
		}

		process, err := os.FindProcess(pid)
		if err != nil {
			logger.Infof("Failed to find process: %v", err)
			os.Exit(0)
		}

		err = process.Signal(syscall.SIGKILL)
		if err != nil {
			logger.Infof("Failed to terminate process: %v", err)
		}

		logger.Infof("Process terminated successfully")
		os.Exit(0)
	}
	return *path
}

func genConfigFile(rootPath string, mode config.NodeMode) error {
	defaultCfg := config.NewDefaultTvbaseConfig()
	prikey, prikeyHex, err := identity.GenIdenity()
	if err != nil {
		return err
	}
	printPriKey(prikey)

	cfg, err := loadConfig(rootPath)
	if err != nil {
		return err
	}
	if cfg == nil {
		cfg = defaultCfg
	}
	cfg.InitMode(mode)
	cfg.Identity.PrivKey = prikeyHex
	// TODO: generate PrivSwarmKey
	// cfg.Identity.PrivSwarmKey = ""
	file, _ := json.MarshalIndent(cfg, "", " ")
	if err := os.WriteFile(rootPath+configFileName, file, 0644); err != nil {
		logger.Infof("failed to WriteFile:", err)
		return err
	}
	logger.Infof("generate config file: " + rootPath + configFileName)
	return nil
}

func printPriKey(privateKey crypto.PrivKey) {
	privateKeyData, _ := crypto.MarshalPrivateKey(privateKey)
	privateKeyStr := base64.StdEncoding.EncodeToString(privateKeyData)
	publicKey := privateKey.GetPublic()
	publicKeyData, _ := crypto.MarshalPublicKey(publicKey)
	publicKeyStr := base64.StdEncoding.EncodeToString(publicKeyData)
	peerId, _ := peer.IDFromPublicKey(publicKey)
	logger.Infof("\nprivateKey: %s\npublicKey: %s\npeerId: %s", privateKeyStr, publicKeyStr, peerId.Pretty())
}
