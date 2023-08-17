package util

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	ipfsLog "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mitchellh/go-homedir"
	ma "github.com/multiformats/go-multiaddr"
	tvbaseConfig "github.com/tinyverse-web3/tvbase/common/config"
	"github.com/tinyverse-web3/tvbase/common/define"
	"github.com/tinyverse-web3/tvbase/common/identity"
)

func GenConfig2IdentityFile(rootPath string, mode define.NodeMode) error {
	rootPath = strings.Trim(rootPath, " ")
	if rootPath == "" {
		rootPath = "."
	}
	fullPath, err := homedir.Expand(rootPath)
	if err != nil {
		fmt.Println("GenConfig2IdentityFile->homedir.Expand: " + err.Error())
		return err
	}
	if !filepath.IsAbs(fullPath) {
		defaultRootPath, err := os.Getwd()
		if err != nil {
			fmt.Println("GenConfig2IdentityFile->Getwd: " + err.Error())
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
			fmt.Println("GenConfig2IdentityFile->MkdirAll: " + err.Error())
			return err
		}
	}

	err = tvbaseConfig.Merge2GenConfigFile(fullPath, mode)
	if err != nil {
		fmt.Println("GenConfig2IdentityFile->GenConfigFile: err:" + err.Error())
	}

	fmt.Println("GenConfig2IdentityFile->generate node config file: " + fullPath + tvbaseConfig.NodeConfigFileName)
	err = identity.GenIdenityFile(fullPath)
	if err != nil {
		fmt.Println("GenConfig2IdentityFile->GenIdenityFile: " + err.Error())
	}
	fmt.Println("GenConfig2IdentityFile->generate identity file: " + fullPath + identity.IdentityFileName)
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

func LoadNodeConfig(options ...any) (*tvbaseConfig.NodeConfig, error) {
	rootPath := ""
	if len(options) > 0 {
		ok := false
		rootPath, ok = options[0].(string)
		if !ok {
			fmt.Println("LoadNodeConfig: options[0](rootPath) is not string")
			return nil, fmt.Errorf("LoadNodeConfig: options[0](rootPath) is not string")
		}
	}
	defaultMode := define.LightMode
	if len(options) > 1 {
		ok := false
		defaultMode, ok = options[1].(define.NodeMode)
		if !ok {
			fmt.Println("LoadNodeConfig: options[0](rootPath) is not string")
			return nil, fmt.Errorf("LoadNodeConfig: options[0](rootPath) is not string")
		}
	}

	fullPath, err := GetRootPath(rootPath)
	if err != nil {
		return nil, err
	}

	config, err := tvbaseConfig.InitNodeConfigFile(fullPath, defaultMode)
	if err != nil {
		return nil, err
	}
	return config, nil
}

func SetLogModule(moduleLevels map[string]string) error {
	var sortedModuleList []string
	for module := range moduleLevels {
		sortedModuleList = append(sortedModuleList, module)
	}
	sort.Strings(sortedModuleList)
	for _, module := range sortedModuleList {
		level := moduleLevels[module]
		err := ipfsLog.SetLogLevelRegex(module, level)
		if err != nil {
			fmt.Printf("SetLogModule->SetLogLevelRegex: %v\n", err)
		}
	}
	return nil
}

// TOOD : need delete
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

// ParseBootstrapPeer parses a bootstrap list into a list of AddrInfos.
func ParseBootstrapPeers(addrs []string) ([]peer.AddrInfo, error) {
	maddrs := make([]ma.Multiaddr, len(addrs))
	for i, addr := range addrs {
		var err error
		maddrs[i], err = ma.NewMultiaddr(addr)
		if err != nil {
			return nil, err
		}
	}
	return peer.AddrInfosFromP2pAddrs(maddrs...)
}
