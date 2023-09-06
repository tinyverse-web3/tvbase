package pullcid

import (
	ipfsLog "github.com/ipfs/go-log/v2"
)

const loggerName = "dmsg.protocol.custom.ipfs"

var log = ipfsLog.Logger(loggerName)

const pid = "ipfs/request/save"

// service
