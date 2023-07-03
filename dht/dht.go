package dht

import (
	"context"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"
	tvConfig "github.com/tinyverse-web3/tvbase/common/config"
	"github.com/tinyverse-web3/tvbase/dkvs"
	db "github.com/tinyverse-web3/tvutil/db"
)

func NewTinyverseNetDht(ctx context.Context, h host.Host, ds db.Datastore, mode tvConfig.NodeMode, protocolPrefix string) (*dht.IpfsDHT, error) {
	var modeCfg dht.Option
	switch mode {
	case tvConfig.FullMode:
		modeCfg = dht.Mode(dht.ModeServer)
	case tvConfig.LightMode:
		modeCfg = dht.Mode(dht.ModeAuto)
	}
	dht, err := dht.New(ctx,
		h,
		dht.ProtocolPrefix(protocol.ID(protocolPrefix)), // dht.ProtocolPrefix("/test"),
		dht.Validator(dkvs.Validator{}),                 // dht.NamespacedValidator("tinyverseNetwork", blankValidator{}),
		modeCfg,
		dht.Datastore(ds),
	)

	return dht, err
}
