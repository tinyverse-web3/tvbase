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

var nodeMode config.NodeMode = config.ServiceMode

// var isTest bool
var env *string
var localnetTestEnv = "test-localnet"
var internetTestEnv = "test-internet"
var defaultEnv = localnetTestEnv

func parseCmdParams() string {
	generateCfg := flag.Bool("init", false, "init generate identity key and config file")
	mode := flag.String("mode", "service", "Initialize tvnode mode for service mode or light mode.")
	rootPath := flag.String("rootPath", defaultPathRoot, "config file path")
	shutDown := flag.Bool("shutdown", false, "shutdown daemon")
	help := flag.Bool("help", false, "Display help")
	// test := flag.Bool("test", false, "Test mode.")
	env = flag.String("env", defaultEnv, "prod or test-localnet or test-internet")

	flag.Parse()

	if *help {
		logger.Info("tinverse tvnode")
		logger.Info("Usage step1: Run './tvnode -init' generate identity key and config.")
		logger.Info("Usage step2: Run './tvnode' or './tvnode -rootPath .' start tinyverse tvnode service.")
		os.Exit(0)
	}

	if *mode == "light" {
		nodeMode = config.LightMode
	}

	// if *test {
	// 	isTest = true
	// }

	if *generateCfg {
		dataPath, err := tvbaseUtil.GetRootPath(*rootPath)
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

		logger.Infof("Generate config file successfully.")
		os.Exit(0)
	}

	if *shutDown {
		pidFile, err := getPidFileName(*rootPath)
		if err != nil {
			logger.Infof("Failed to get pidFileName: %v", err)
			os.Exit(0)
		}
		file, err := os.Open(pidFile)
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
	return *rootPath
}

func genConfigFile(rootPath string, mode config.NodeMode) error {
	cfg := config.NewDefaultTvNodeConfig()
	prikey, prikeyHex, err := identity.GenIdenity()
	if err != nil {
		return err
	}
	printPriKey(prikey)
	cfg.Tvbase.InitMode(mode)
	cfg.Tvbase.Identity.PrivKey = prikeyHex

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
