package custom

import (
	"github.com/tinyverse-web3/tvbase/dmsg/protocol"
)

type CustomStreamProtocolClientInfo struct {
	Client   CustomStreamProtocolClient
	Protocol *protocol.StreamProtocol
}
type CustomStreamProtocolServiceInfo struct {
	Service  CustomStreamProtocolService
	Protocol *protocol.StreamProtocol
}
