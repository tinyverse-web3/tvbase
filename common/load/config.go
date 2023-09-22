package load

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"

	"github.com/tinyverse-web3/tvbase/common/config"
	"github.com/tinyverse-web3/tvbase/common/identity"
)

const (
	ConfigFileName = "config.json"
)

func LoadConfig(rootPath string) (*config.TvNodeConfig, error) {
	ret := &config.TvNodeConfig{}
	configFilePath := rootPath + ConfigFileName
	_, err := os.Stat(configFilePath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	err = ret.LoadConfig(configFilePath)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func GenConfigFile(rootPath string, mode config.NodeMode) error {
	defaultCfg := config.NewDefaultTvNodeConfig()
	cfg, err := LoadConfig(rootPath)
	if err != nil {
		return err
	}
	if cfg == nil {
		cfg = defaultCfg
	} else {
		err := mergeJSON(cfg, defaultCfg)
		if err != nil {
			return err
		}
	}
	cfg.Tvbase.InitMode(mode)
	file, _ := json.MarshalIndent(cfg, "", " ")
	if err := os.WriteFile(rootPath+ConfigFileName, file, 0644); err != nil {
		fmt.Println("CreateConfigFileIfNotExist: Failed to WriteFile:", err)
		return err
	}
	fmt.Println("genConfigFile->generate node config file: " + rootPath + ConfigFileName)
	return nil
}

func GenIdentityFile(rootPath string) error {
	err := identity.GenIdenityFile(rootPath)
	if err != nil {
		fmt.Println("GenConfig2IdentityFile->GenIdenityFile: " + err.Error())
	}
	fmt.Println("GenConfig2IdentityFile->generate identity file: " + rootPath + identity.IdentityFileName)
	return nil
}

func mergeJSON(srcObj, destObj any) error {
	srcBytes, _ := json.Marshal(srcObj)
	destBytes, _ := json.Marshal(destObj)

	var srcMap map[string]any
	var destMap map[string]any
	err := json.Unmarshal(srcBytes, &srcMap)
	if err != nil {
		return err
	}
	err = json.Unmarshal(destBytes, &destMap)
	if err != nil {
		return err
	}

	mergeJsonFields(srcMap, destMap)

	result, err := json.Marshal(srcMap)
	if err != nil {
		return err
	}
	err = json.Unmarshal(result, &srcObj)
	if err != nil {
		return err
	}
	return nil
}

func mergeJsonFields(srcObj, destObj map[string]any) error {
	for key, value := range destObj {
		if value == nil {
			continue
		}
		if reflect.TypeOf(value).Kind() == reflect.Map {
			if srcObj[key] == nil {
				srcObj[key] = make(map[string]any)
			}
			err := mergeJsonFields(srcObj[key].(map[string]any), value.(map[string]any))
			if err != nil {
				return err
			}
		} else {
			if srcObj[key] == nil {
				srcObj[key] = value
			}
		}
	}
	return nil
}
