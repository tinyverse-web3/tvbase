package dht

import (
	"context"

	"github.com/libp2p/go-libp2p"
	kaddht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/routing"
	tvConfig "github.com/tinyverse-web3/tvbase/common/config"
	tvLog "github.com/tinyverse-web3/tvbase/common/log"
	db "github.com/tinyverse-web3/tvutil/db"
)

func CreateOpts(ctx context.Context, dht **kaddht.IpfsDHT, ds db.Datastore, nodeMode tvConfig.NodeMode, protocolPrefix string) ([]libp2p.Option, error) {
	var opts []libp2p.Option
	opt, err := createRoutingOpt(ctx, dht, ds, nodeMode, protocolPrefix)
	if err != nil {
		tvLog.Logger.Errorf("infrasture->createOpts: error: %v", err)
		return nil, err
	}
	opts = append(opts, opt)
	return opts, nil
}

func createRoutingOpt(ctx context.Context, dht **kaddht.IpfsDHT, ds db.Datastore, nodeMode tvConfig.NodeMode, protocolPrefix string) (libp2p.Option, error) {
	var err error
	opt := libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {
		*dht, err = NewTinyverseNetDht(ctx, h, ds, nodeMode, protocolPrefix)
		return *dht, err
	})
	return opt, nil
}
