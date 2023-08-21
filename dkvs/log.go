package dkvs

import (
	"github.com/ipfs/go-log/v2"
)

var Logger *log.ZapEventLogger

func init() {
	Logger = log.Logger(DKVS_NAMESPACE)
}
