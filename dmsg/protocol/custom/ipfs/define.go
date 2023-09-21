package syncfile

import (
	ipfsLog "github.com/ipfs/go-log/v2"
)

const loggerName = "dmsg.protocol.custom.ipfs"

var logger = ipfsLog.Logger(loggerName)

const PID_SERVICE_SYNCFILE_UPLOAD = "tvSyncFileUploadService"
const PID_SERVICE_SYNCFILE_SUMMARY = "tvSyncFileSummaryService"

const (
	CODE_SUCC           = 0
	CODE_ALREADY_PIN    = 1
	CODE_ERROR_NOPIN    = -1
	CODE_ERROR_IPFS     = -2
	CODE_ERROR_PROTOCOL = -3
	CODE_ERROR_PROVIDER = -4
)
