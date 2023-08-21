package protocol

import (
	"context"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"google.golang.org/protobuf/proto"
)

func SendProtocolMsg(ctx context.Context, host host.Host, peerID peer.ID, pid protocol.ID, protoData proto.Message) error {
	stream, err := host.NewStream(ctx, peerID, pid)
	if err != nil {
		return err
	}
	defer func() {
		if stream != nil {
			stream.Close()
		}
	}()

	buf, err := proto.Marshal(protoData)
	if err != nil {
		return err
	}
	_, err = stream.Write(buf)
	if err != nil {
		stream.Reset()
		stream = nil
		return err
	}
	return nil
}
