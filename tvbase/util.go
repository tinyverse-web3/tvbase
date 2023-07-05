package tvbase

import ipfsLog "github.com/ipfs/go-log/v2"

func SetDegbugMode(lv string) {
	interalModuleNameList := []string{
		"tvbase",
		"dkvs",
		"dmsg",
		"customProtocol",
	}
	for _, module := range interalModuleNameList {
		ipfsLog.SetLogLevel(module, lv)
	}
}
