package tvbase

import ipfsLog "github.com/ipfs/go-log/v2"

func SetDebugMode(lv string, moreModuleList ...string) {
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
