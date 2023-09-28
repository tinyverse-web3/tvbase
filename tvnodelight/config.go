package main

import (
	"os"

	"github.com/tinyverse-web3/tvbase/common/config"
)

const (
	configFileName = "config.json"
)

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
