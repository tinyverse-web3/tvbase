package custom

import (
	"github.com/tinyverse-web3/tvbase/dmsg/protocol"
)

type ClientStreamProtocol struct {
	Handle   ClientHandle
	Protocol *protocol.CustomSProtocol
}
type ServerStreamProtocol struct {
	Handle   ServerHandle
	Protocol *protocol.CustomSProtocol
}
