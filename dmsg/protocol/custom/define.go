package custom

import (
	"github.com/tinyverse-web3/tvbase/dmsg/protocol"
)

type CustomStreamProtocolInfo struct {
	Client   CustomStreamProtocolClient
	Protocol *protocol.StreamProtocol
}

type CustomPubsubProtocolInfo struct {
	Client   CustomPubsubProtocolClient
	Protocol *protocol.PubsubProtocol
}
