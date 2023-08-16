package util

import (
	"context"
	"math/rand"
	"time"

	b4 "github.com/cenkalti/backoff/v4"
	"github.com/libp2p/go-libp2p/core/discovery"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/libp2p/go-libp2p/p2p/discovery/backoff"
	drouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	tvLog "github.com/tinyverse-web3/tvbase/common/log"
)

// use fullJitter Exponential backoff to optimize the discovery algorithm
func CreateDiscovery(host host.Host, cr routing.ContentRouting) (service discovery.Discovery, err error) {
	baseDisc := drouting.NewRoutingDiscovery(cr)
	minBackoff, maxBackoff := time.Second*60, time.Hour
	rng := rand.New(rand.NewSource(rand.Int63()))
	discovery, err := backoff.NewBackoffDiscovery(
		baseDisc,
		backoff.NewExponentialBackoff(minBackoff, maxBackoff, backoff.FullJitter, time.Second, 5.0, 0, rng),
	)
	if err != nil {
		return nil, err
	}
	return discovery, nil
}

// @REF: github.com/libp2p/go-libp2p/p2p/discovery/util:PubsubAdvertise()
func PubsubAdvertise(ctx context.Context, a discovery.Advertiser, ns string, opts ...discovery.Option) {
	go func() {
		bo := b4.NewExponentialBackOff()
		bo.InitialInterval = 2 * time.Second
		bo.Multiplier = 1.5
		bo.MaxInterval = 30 * time.Second
		bo.MaxElapsedTime = 0 // never stop
		t := b4.NewTicker(bo)

		for {
			ttl, err := a.Advertise(ctx, ns, opts...)

			if err != nil {
				tvLog.Logger.Warnf("Advertising failure, ns:%s, err: %v", ns, err)
				if ctx.Err() != nil {
					return
				}

				select {
				case <-t.C:
					continue
				// case <-time.After(30 * time.Second):
				// 	continue
				case <-ctx.Done():
					return
				}
			}
			tvLog.Logger.Infof("Advertising success, ns:%s, ttl:%v", ns, ttl)

			t.Stop()
			wait := 7 * ttl / 8
			if wait > 10*time.Minute {
				wait = 10 * time.Minute
			}

			select {
			case <-time.After(wait):
				t = b4.NewTicker(bo)
			case <-ctx.Done():
				return
			}
		}
	}()
}
