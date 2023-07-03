package dkvs

import (
	"github.com/ipfs/go-log/v2"
)

const (
	LogDebug  = "debug"
	LogInfo   = "info"
	LogWarn   = "warn"
	LogError  = "error"
	LogDpanic = "dpanic"
	LogPanic  = "panic" // DPanicLevel logs are particularly important errors. In development the logger panics after writing the message.
	LogFatal  = "fatal"
)

var Logger *log.ZapEventLogger

func init() {
	Logger = log.Logger(DKVS_NAMESPACE)
}

// func InitAPP(level string) {
// 	mtvlog.InitAPP(level)
// }
// func InitModule(moduleName string, level string) {
// 	Logger = mtvlog.InitModule(moduleName, level)
// }
