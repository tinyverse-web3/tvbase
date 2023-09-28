package main

import (
	ipfsLog "github.com/ipfs/go-log/v2"
	"github.com/tinyverse-web3/tvbase/common/util"
)

const (
	logName = "tvnodelight"
)

var logger = ipfsLog.Logger(logName)

func initLog() (err error) {
	var moduleLevels = map[string]string{
		"tvbase":         "debug",
		"dkvs":           "debug",
		"dmsg":           "debug",
		"customProtocol": "debug",
		"tvnode":         "debug",
		"tvipfs":         "debug",
		"core_http":      "debug",
	}
	err = util.SetLogModule(moduleLevels)
	if err != nil {
		return err
	}
	return nil
}
