package tvbase

import (
	"os"

	"github.com/ipfs/kubo/config"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/p2p/muxer/yamux"
	"github.com/tinyverse-web3/tvbase/tvbase/internal/mplex"
)

func yamuxTransport() network.Multiplexer {
	tpt := *yamux.DefaultTransport
	tpt.AcceptBacklog = 512
	if os.Getenv("YAMUX_DEBUG") != "" {
		tpt.LogOutput = os.Stderr
	}
	return &tpt
}

func makeSmuxTransportOption(tptConfig config.Transports) (libp2p.Option, error) {
	const yamuxID = "/yamux/1.0.0"
	const mplexID = "/mplex/6.7.0"

	return prioritizeOptions([]priorityOption{{
		priority:        tptConfig.Multiplexers.Yamux,
		defaultPriority: 100,
		opt:             libp2p.Muxer(yamuxID, yamuxTransport()),
	}, {
		priority:        tptConfig.Multiplexers.Mplex,
		defaultPriority: 200,
		opt:             libp2p.Muxer(mplexID, mplex.DefaultTransport),
	}}), nil

}
