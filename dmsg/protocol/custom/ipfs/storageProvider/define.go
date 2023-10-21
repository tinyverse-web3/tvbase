package storageprovider

import (
	"time"

	ipfsLog "github.com/ipfs/go-log/v2"
)

const (
	loggerName = "dmsg.protocol.custom.ipfs.storageProvider"
)

var (
	logger = ipfsLog.Logger(loggerName)

	CommonTimeout = 30 * time.Second
)
