package channel

import (
	"time"

	"github.com/tinyverse-web3/tvbase/common/define"
	dmsgKey "github.com/tinyverse-web3/tvbase/dmsg/common/key"

	"github.com/tinyverse-web3/tvbase/dmsg/protocol/adapter"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/basic"
)

type ChannelService struct {
	ChannelBase
	createPubsubProtocol *basic.CreatePubsubSProtocol
	pubsubMsgProtocol    *basic.PubsubMsgProtocol
	pubkey               string
	getSig               dmsgKey.GetSigCallback
}

func NewService(tvbase define.TvBaseService, pubkey string, getSig dmsgKey.GetSigCallback) (*ChannelService, error) {
	d := &ChannelService{}
	err := d.Init(tvbase, pubkey, getSig)
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (d *ChannelService) Init(tvbase define.TvBaseService, pubkey string, getSig dmsgKey.GetSigCallback) error {
	cfg := tvbase.GetConfig().DMsg
	err := d.ChannelBase.Init(tvbase, cfg.MaxChannelCount, cfg.KeepChannelDay)
	if err != nil {
		return err
	}

	d.pubkey = pubkey
	d.getSig = getSig
	return nil
}

// sdk-common
func (d *ChannelService) Start() error {
	log.Debug("ChannelService->Start begin")
	ctx := d.TvBase.GetCtx()
	host := d.TvBase.GetHost()

	if d.createPubsubProtocol == nil {
		d.createPubsubProtocol = adapter.NewCreateChannelProtocol(ctx, host, d, d, true, d.pubkey)
	}

	if d.pubsubMsgProtocol == nil {
		d.pubsubMsgProtocol = adapter.NewPubsubMsgProtocol(ctx, host, d, d)
		d.RegistPubsubProtocol(d.pubsubMsgProtocol.Adapter.GetRequestPID(), d.pubsubMsgProtocol)
		d.RegistPubsubProtocol(d.pubsubMsgProtocol.Adapter.GetResponsePID(), d.pubsubMsgProtocol)
	}

	if d.createPubsubProtocol != nil && d.pubsubMsgProtocol != nil {
		err := d.ProxyPubsubService.Start(d.pubkey, d.getSig, d.createPubsubProtocol, d.pubsubMsgProtocol, false)
		if err != nil {
			return err
		}
	}

	d.CleanRestPubsub(12 * time.Hour)

	d.enable = true
	log.Debug("ChannelService->Start end")
	return nil
}

func (d *ChannelService) Stop() error {
	d.enable = false
	return nil
}

func (d *ChannelService) Release() error {
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

	err = d.Stop()
	if err != nil {
		return err
	}
	return nil
}
