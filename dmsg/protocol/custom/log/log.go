package log

import (
	ipfsLog "github.com/ipfs/go-log/v2"
)

const LoggerName = "dmsg.protocol.custom"

var Logger = ipfsLog.Logger(LoggerName)
