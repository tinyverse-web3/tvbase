package channel

import (
	"github.com/tinyverse-web3/tvbase/common/define"
	dmsgKey "github.com/tinyverse-web3/tvbase/dmsg/common/key"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/adapter"
)

type ChannelClient struct {
	ChannelBase
}

func CreateClient(tvbase define.TvBaseService) (*ChannelClient, error) {
	d := &ChannelClient{}
	err := d.Init(tvbase, 10000, 365)
	if err != nil {
		return nil, err
	}
	return d, nil
}

// sdk-common
func (d *ChannelClient) Start(pubkey string, getSig dmsgKey.GetSigCallback) error {
	log.Debug("ChannelClient->Start begin")
	ctx := d.TvBase.GetCtx()
	host := d.TvBase.GetHost()

	createPubsubProtocol := adapter.NewCreateChannelProtocol(ctx, host, d, d, false, pubkey)
	pubsubMsgProtocol := adapter.NewPubsubMsgProtocol(ctx, host, d, d)
	d.RegistPubsubProtocol(pubsubMsgProtocol.Adapter.GetRequestPID(), pubsubMsgProtocol)
	d.RegistPubsubProtocol(pubsubMsgProtocol.Adapter.GetResponsePID(), pubsubMsgProtocol)
	err := d.ProxyPubsubService.Start(pubkey, getSig, createPubsubProtocol, pubsubMsgProtocol, false)
	if err != nil {
		return err
	}

	log.Debug("ChannelClient->Start end")
	return nil
}
