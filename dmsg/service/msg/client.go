package msg

import (
	"fmt"

	"github.com/tinyverse-web3/tvbase/common/define"
	dmsgKey "github.com/tinyverse-web3/tvbase/dmsg/common/key"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/adapter/pubsub"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/adapter/stream"
)

type MsgClient struct {
	MsgBase
}

func NewClient(tvbase define.TvBaseService, pubkey string, getSig dmsgKey.GetSigCallback, isListenMsg bool) (*MsgClient, error) {
	d := &MsgClient{}
	err := d.Init(tvbase, pubkey, getSig, isListenMsg)
	if err != nil {
		return nil, err
	}
	return d, nil
}

// sdk-common
func (d *MsgClient) Init(tvbase define.TvBaseService, pubkey string, getSig dmsgKey.GetSigCallback, isListenMsg bool) error {
	log.Debugf("MsgClient->Init begin")

	cfg := tvbase.GetConfig().DMsg
	err := d.MsgBase.Init(tvbase, cfg.MaxMsgCount, cfg.KeepMsgDay)
	if err != nil {
		return err
	}

	ctx := d.TvBase.GetCtx()
	host := d.TvBase.GetHost()

	if d.createPubsubProtocol == nil {
		d.createPubsubProtocol = stream.NewCreateMsgPubsubProtocol(ctx, host, d, d, false, pubkey)
	}
	if d.pubsubMsgProtocol == nil {
		d.pubsubMsgProtocol = pubsub.NewPubsubMsgProtocol(ctx, host, d, d)
		d.RegistPubsubProtocol(d.pubsubMsgProtocol.Adapter.GetRequestPID(), d.pubsubMsgProtocol)
		d.RegistPubsubProtocol(d.pubsubMsgProtocol.Adapter.GetResponsePID(), d.pubsubMsgProtocol)
	}

	if d.createPubsubProtocol != nil && d.pubsubMsgProtocol != nil {
		err = d.ProxyPubsubService.Start(pubkey, getSig, d.createPubsubProtocol, d.pubsubMsgProtocol, isListenMsg)
		if err != nil {
			return err
		}
	}
	d.enable = true
	log.Debug("MsgClient->Init end")
	return nil
}

func (d *MsgClient) Release() error {
	//TODO
	// d.createPubsubProtocol.Release()

	d.UnregistPubsubProtocol(d.createPubsubProtocol.Adapter.GetRequestPID())
	d.UnregistPubsubProtocol(d.createPubsubProtocol.Adapter.GetResponsePID())
	d.pubsubMsgProtocol = nil
	d.createPubsubProtocol = nil

	d.enable = false
	return nil
}

func (d *MsgClient) SetProxy(pubkey string) error {
	if pubkey == "" {
		return fmt.Errorf("MsgClient->SetProxy: pubkey is empty")
	}
	if d.GetProxyPubkey() != "" {
		return fmt.Errorf("MsgClient->SetProxy: proxyPubkey is not empty")
	}
	err := d.SubscribeDestUser(pubkey, false)
	if err != nil {
		return err
	}
	err = d.BaseService.SetProxyPubkey(pubkey)
	if err != nil {
		return err
	}
	return nil
}

func (d *MsgClient) ClearProxy() error {
	proxyPubkey := d.GetProxyPubkey()
	if proxyPubkey != "" {
		err := d.UnSubscribeDestUser(proxyPubkey)
		if err != nil {
			return err
		}
		d.SetProxyPubkey("")
	}
	return nil
}
