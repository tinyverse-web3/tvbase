package custom

import (
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/basic"
)

type ClientStreamProtocol struct {
	Handle   ClientHandle
	Protocol *basic.CustomSProtocol
}
type ServerStreamProtocol struct {
	Handle   ServerHandle
	Protocol *basic.CustomSProtocol
}
