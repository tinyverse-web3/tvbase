package syncfile

import (
	ipfsLog "github.com/ipfs/go-log/v2"
)

const loggerName = "dmsg.protocol.custom.ipfs"

var logger = ipfsLog.Logger(loggerName)

const TV_SYNCFILE_UPLOAD_SERVICE = "tvSyncFileUploadService"
const TV_SYNCFILE_SUMMARY_SERVICE = "tvSyncFileSummaryService"

const (
	CODE_PIN            = 0
	CODE_ALREADY_PIN    = 1
	CODE_ERROR_NOPIN    = -1
	CODE_ERROR_IPFS     = -2
	CODE_ERROR_PROTOCOL = -3
	CODE_ERROR_PROVIDER = -4
)
