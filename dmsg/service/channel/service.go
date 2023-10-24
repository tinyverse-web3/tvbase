package channel

import (
	"time"

	"github.com/tinyverse-web3/tvbase/common/define"
	dmsgKey "github.com/tinyverse-web3/tvbase/dmsg/common/key"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/adapter"
)

type ChannelService struct {
	ChannelBase
}

func CreateService(tvbase define.TvBaseService) (*ChannelService, error) {
	d := &ChannelService{}
	cfg := tvbase.GetConfig().DMsg
	err := d.Init(tvbase, cfg.MaxChannelCount, cfg.KeepChannelDay)
	if err != nil {
		return nil, err
	}
	return d, nil
}

// sdk-common
func (d *ChannelService) Start(
	pubkey string,
	getSig dmsgKey.GetSigCallback,
) error {
	log.Debug("ChannelService->Start begin")
	ctx := d.TvBase.GetCtx()
	host := d.TvBase.GetHost()

	createPubsubProtocol := adapter.NewCreateChannelProtocol(ctx, host, d, d, true, pubkey)
	pubsubMsgProtocol := adapter.NewPubsubMsgProtocol(ctx, host, d, d)
	d.RegistPubsubProtocol(pubsubMsgProtocol.Adapter.GetRequestPID(), pubsubMsgProtocol)
	d.RegistPubsubProtocol(pubsubMsgProtocol.Adapter.GetResponsePID(), pubsubMsgProtocol)
	err := d.ProxyPubsubService.Start(pubkey, getSig, createPubsubProtocol, pubsubMsgProtocol, false)
	if err != nil {
		return err
	}

	d.CleanRestPubsub(12 * time.Hour)

	log.Debug("ChannelService->Start end")
	return nil
}
