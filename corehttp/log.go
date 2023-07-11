package corehttp

import (
	ipfsLog "github.com/ipfs/go-log/v2"
)

var Logger *ipfsLog.ZapEventLogger

func InitLog() {
	Logger = ipfsLog.Logger("core_http")
	ipfsLog.SetLogLevel("core_http", "debug")
}
