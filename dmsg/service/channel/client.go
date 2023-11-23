package channel

import (
	"github.com/tinyverse-web3/tvbase/common/define"
	dmsgKey "github.com/tinyverse-web3/tvbase/dmsg/common/key"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/adapter/pubsub"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/adapter/stream"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/basic"
)

type ChannelClient struct {
	ChannelBase
	createPubsubProtocol *basic.CreatePubsubSProtocol
	pubsubMsgProtocol    *basic.PubsubMsgProtocol
}

func NewClient(tvbase define.TvBaseService, pubkey string, getSig dmsgKey.GetSigCallback) (*ChannelClient, error) {
	d := &ChannelClient{}

	err := d.Init(tvbase, pubkey, getSig)
	if err != nil {
		return nil, err
	}
	return d, nil
}

// sdk-common
func (d *ChannelClient) Init(tvbase define.TvBaseService, pubkey string, getSig dmsgKey.GetSigCallback) error {
	log.Debug("ChannelClient->Start begin")
	err := d.ChannelBase.Init(tvbase, 10000, 365)
	if err != nil {
		return err
	}

	ctx := d.TvBase.GetCtx()
	host := d.TvBase.GetHost()

	if d.createPubsubProtocol == nil {
		d.createPubsubProtocol = stream.NewCreateChannelProtocol(ctx, host, d, d, false, pubkey)
	}

	if d.pubsubMsgProtocol == nil {
		d.pubsubMsgProtocol = pubsub.NewPubsubMsgProtocol(ctx, host, d, d)
		d.RegistPubsubProtocol(d.pubsubMsgProtocol.Adapter.GetRequestPID(), d.pubsubMsgProtocol)
		d.RegistPubsubProtocol(d.pubsubMsgProtocol.Adapter.GetResponsePID(), d.pubsubMsgProtocol)
	}

	if d.createPubsubProtocol != nil && d.pubsubMsgProtocol != nil {
		err = d.ProxyPubsubService.Start(pubkey, getSig, d.createPubsubProtocol, d.pubsubMsgProtocol, false)
		if err != nil {
			return err
		}
	}

	d.enable = true
	log.Debug("ChannelClient->Start end")
	return nil
}

func (d *ChannelClient) Release() error {
	log.Debug("ChannelClient->Release begin")
	// TODO
	// d.createPubsubProtocol.Release()
	d.createPubsubProtocol = nil

	d.UnregistPubsubProtocol(d.pubsubMsgProtocol.Adapter.GetRequestPID())
	d.UnregistPubsubProtocol(d.pubsubMsgProtocol.Adapter.GetResponsePID())
	d.pubsubMsgProtocol = nil

	err := d.ProxyPubsubService.Stop()
	if err != nil {
		return err
	}

	d.enable = false
	log.Debug("ChannelClient->Release end")
	return nil
}
