package tvbase

import (
	"fmt"
	"os"
	"strings"

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

	if prefs := os.Getenv("LIBP2P_MUX_PREFS"); prefs != "" {
		// Using legacy LIBP2P_MUX_PREFS variable.
		fmt.Println("LIBP2P_MUX_PREFS is now deprecated.")
		fmt.Println("Use the `Swarm.Transports.Multiplexers' config field.")
		muxers := strings.Fields(prefs)
		enabled := make(map[string]bool, len(muxers))

		var opts []libp2p.Option
		for _, tpt := range muxers {
			if enabled[tpt] {
				return nil, fmt.Errorf(
					"duplicate muxer found in LIBP2P_MUX_PREFS: %s",
					tpt,
				)
			}
			switch tpt {
			case yamuxID:
				opts = append(opts, libp2p.Muxer(tpt, yamuxTransport()))
			case mplexID:
				opts = append(opts, libp2p.Muxer(tpt, mplex.DefaultTransport))
			default:
				return nil, fmt.Errorf("unknown muxer: %s", tpt)
			}
		}
		return libp2p.ChainOptions(opts...), nil
	} else {
		return prioritizeOptions([]priorityOption{{
			priority:        tptConfig.Multiplexers.Yamux,
			defaultPriority: 100,
			opt:             libp2p.Muxer(yamuxID, yamuxTransport()),
		},
		// {
		// 	priority:        tptConfig.Multiplexers.Mplex,
		// 	defaultPriority: 200,
		// 	opt:             libp2p.Muxer(mplexID, mplex.DefaultTransport),
		// },
		}), nil
	}
}
