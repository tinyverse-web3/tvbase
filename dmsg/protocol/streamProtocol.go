package protocol

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/protoio"
	"google.golang.org/protobuf/proto"
)

func SendProtocolMsg(ctx context.Context, id peer.ID, p protocol.ID, data proto.Message, host host.Host) error {
	fmt.Printf("sendProtocolMsg:%v/n", data)
	stream, err := host.NewStream(ctx, id, p)
	if err != nil {
		return err
	}
	defer func() {
		if stream != nil {
			stream.Close()
		}
	}()

	writer := protoio.NewFullWriter(stream)
	err = writer.WriteMsg(data)
	if err != nil {
		stream.Reset()
		stream = nil
		return err
	}
	return nil
}
