package util

import (
	"sort"

	ipfsLog "github.com/ipfs/go-log/v2"
)

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
			return err
		}
	}
	return nil
}
