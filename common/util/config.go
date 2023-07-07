package util

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	ipfsLog "github.com/ipfs/go-log/v2"
	"github.com/mitchellh/go-homedir"
	tvConfig "github.com/tinyverse-web3/tvbase/common/config"
	"github.com/tinyverse-web3/tvbase/common/identity"
	"github.com/tinyverse-web3/tvbase/common/log"
)

func GenConfig2IdentityFile(rootPath string, mode tvConfig.NodeMode) error {
	rootPath = strings.Trim(rootPath, " ")
	if rootPath == "" {
		rootPath = "."
	}
	fullPath, err := homedir.Expand(rootPath)
	if err != nil {
		return err
	}
	if !filepath.IsAbs(fullPath) {
		defaultRootPath, err := os.Getwd()
		if err != nil {
			return err
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
			fmt.Println("InitConfig: Failed to create directory:", err)
			return err
		}
	}

	config := tvConfig.NewDefaultNodeConfig()

	oldMode := config.Mode
	config.Mode = mode
	err = tvConfig.GenConfigFile(fullPath, &config)
	if err != nil {
		log.Logger.Errorf("generate nodeConfig err: %v", err)
	}
	config.Mode = oldMode
	log.Logger.Infof("already generate identityKey and config file, please run program again.")

	err = identity.GenIdenityFile(fullPath)
	if err != nil {
		log.Logger.Errorf("generate identity err: %v", err)
	}
	return nil
}

func GetRootPath(path string) (string, error) {
	fullPath, err := homedir.Expand(path)
	if err != nil {
		return fullPath, err
	}
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

func LoadNodeConfig(options ...any) (*tvConfig.NodeConfig, error) {
	rootPath := ""
	if len(options) > 0 {
		ok := false
		rootPath, ok = options[0].(string)
		if !ok {
			fmt.Println("LoadNodeConfig: options[0](rootPath) is not string")
			return nil, fmt.Errorf("LoadNodeConfig: options[0](rootPath) is not string")
		}
	}
	config := tvConfig.NewDefaultNodeConfig()

	fullPath, err := GetRootPath(rootPath)
	if err != nil {
		return nil, err
	}
	err = tvConfig.InitConfig(fullPath, &config)
	if err != nil {
		fmt.Println("InitConfig: err:" + err.Error())
		return nil, err
	}
	return nil, nil
}

func SetLogModule(moduleLevels map[string]string) error {
	// ipfsLog.SetAllLoggers(config.Log.AllLogLevel)
	for module, level := range moduleLevels {
		ipfsLog.SetLogLevel(module, level)
	}
	return nil
}

func SetLogLevel(lv string, moreModuleList ...string) {
	interalModuleList := []string{
		"tvbase",
		"dkvs",
		"dmsg",
		"customProtocol",
	}
	for _, module := range interalModuleList {
		ipfsLog.SetLogLevel(module, lv)
	}
	for _, module := range moreModuleList {
		ipfsLog.SetLogLevel(module, lv)
	}
}
