package util

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	ipfsLog "github.com/ipfs/go-log/v2"
	tvConfig "github.com/tinyverse-web3/tvbase/common/config"
	"github.com/tinyverse-web3/tvbase/common/identity"
	"github.com/tinyverse-web3/tvbase/common/log"
)

func GenConfig2IdentityFile(rootPath string, mode tvConfig.NodeMode) error {
	if rootPath != "" && !strings.HasSuffix(rootPath, string(os.PathSeparator)) {
		rootPath += string(os.PathSeparator)
	}
	oldMode := tvConfig.DefaultNodeCfg.Mode
	tvConfig.DefaultNodeCfg.Mode = mode
	err := tvConfig.GenConfigFile(rootPath, &tvConfig.DefaultNodeCfg)
	if err != nil {
		log.Logger.Infoln("generate nodeConfig err: " + err.Error())
	}
	tvConfig.DefaultNodeCfg.Mode = oldMode
	log.Logger.Infof("already generate identityKey and config file, please run program again.\n")

	err = identity.GenIdenityFile2Print(rootPath)
	if err != nil {
		log.Logger.Infoln("generate identity err: " + err.Error())
	}
	return nil
}

func GetRootPath(path string) (string, error) {
	fullPath := path
	if !filepath.IsAbs(fullPath) {
		defaultRootPath, err := os.Getwd()
		if err != nil {
			return fullPath, err
		}
		fullPath = filepath.Join(defaultRootPath, fullPath)
	}
	if !strings.HasSuffix(fullPath, string(os.PathSeparator)) {
		fullPath += string(os.PathSeparator)
	}
	return fullPath, nil
}

func InitConfig(options ...any) error {
	rootPath := ""
	if len(options) > 0 {
		ok := false
		rootPath, ok = options[0].(string)
		if !ok {
			fmt.Println("InitConfig: options[0](rootPath) is not string")
			return fmt.Errorf("InitConfig: options[0](rootPath) is not string")
		}
	}
	config := &tvConfig.DefaultNodeCfg
	fullPath, err := GetRootPath(rootPath)
	if err != nil {
		return err
	}
	err = tvConfig.InitConfig(fullPath, config)
	if err != nil {
		fmt.Println("InitConfig: " + err.Error())
		return err
	}
	return nil
}

func InitLog(options ...any) error {
	config := &tvConfig.DefaultNodeCfg
	if len(options) > 0 {
		ok := false
		config, ok = options[0].(*tvConfig.NodeConfig)
		if !ok {
			fmt.Println("InitLog: options[0](rootPath) is not string")
			return fmt.Errorf("InitLog: options[0](rootPath) is not string")
		}
	}

	ipfsLog.SetAllLoggers(config.Log.AllLogLevel)
	for module, level := range config.Log.ModuleLevels {
		ipfsLog.SetLogLevel(module, level)
	}
	return nil
}
