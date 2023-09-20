package syncfile

import (
	ipfsLog "github.com/ipfs/go-log/v2"
)

const loggerName = "dmsg.protocol.custom.ipfs"

var logger = ipfsLog.Logger(loggerName)

const SYNCIPFSFILEPID = "tv_syncipfsfile"
const CID_STATUS_SERVICE = "cidStatusService"
const PIN_SERVICE = "pinService"

const (
	CODE_PIN            = 0
	CODE_ALREADY_PIN    = 1
	CODE_ERROR_NOPIN    = -1
	CODE_ERROR_IPFS     = -2
	CODE_ERROR_PROTOCOL = -3
	CODE_ERROR_PROVIDER = -4
)
