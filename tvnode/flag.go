package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/tinyverse-web3/tvbase/common/config"
	"github.com/tinyverse-web3/tvbase/common/identity"
	tvbaseUtil "github.com/tinyverse-web3/tvbase/common/util"
)

const (
	defaultPathName = ".tvnode"
	defaultPathRoot = "~/" + defaultPathName
)

func parseCmdParams() string {
	generateCfg := flag.Bool("init", false, "init generate identity key and config file")
	rootPath := flag.String("rootPath", defaultPathRoot, "config file path")
	shutDown := flag.Bool("shutdown", false, "shutdown daemon")
	help := flag.Bool("help", false, "Display help")

	flag.Parse()

	if *help {
		logger.Info("tinverse tvnode")
		logger.Info("Usage step1: Run './tvnode -init' generate identity key and config.")
		logger.Info("Usage step2: Run './tvnode' or './tvnode -rootPath .' start tinyverse tvnode service.")
		os.Exit(0)
	}
	if *generateCfg {
		fullPath, err := tvbaseUtil.GetRootPath(*rootPath)
		if err != nil {
			logger.Fatalf("GetRootPath error: %v", err)
		}
		_, err = os.Stat(fullPath)
		if os.IsNotExist(err) {
			err := os.MkdirAll(fullPath, 0755)
			if err != nil {
				logger.Fatalf("MkdirAll error: %v", err)
			}
		}
		err = genConfigFile(fullPath, config.ServiceMode)
		if err != nil {
			logger.Fatalf("Failed to generate config file: %v", err)
		}
		err = genIdentityFile(fullPath)
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
