package customProtocol

import (
	ipfsLog "github.com/ipfs/go-log/v2"
)

const LoggerName = "customProtocol"

var Logger = ipfsLog.Logger(LoggerName)
