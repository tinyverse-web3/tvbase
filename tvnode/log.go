package main

import (
	ipfsLog "github.com/ipfs/go-log/v2"
	"github.com/tinyverse-web3/tvbase/common/util"
)

const (
	logName = "tvnode"
)

var logger = ipfsLog.Logger(logName)

func init() {
	ipfsLog.SetLogLevelRegex(logName, "info")
}

func initLog() (err error) {
	var moduleLevels = map[string]string{
		"core_http":                "debug",
		"customProtocol":           "debug",
		"dkvs":                     "panic",
		"dmsg":                     "debug",
		"dmsg.common":              "debug",
		"dmsg.protocol":            "debug",
		"dmsg.service.base":        "debug",
		"dmsg.service.channel":     "debug",
		"dmsg.service.mail":        "debug",
		"dmsg.service.msg":         "debug",
		"dmsg.service.proxypubsub": "debug",
		"tvbase":                   "debug",
		"tvipfs":                   "debug",
	}
	err = util.SetLogModule(moduleLevels)
	if err != nil {
		return err
	}
	return nil
}
