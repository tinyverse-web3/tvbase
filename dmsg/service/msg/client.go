package msg

import (
	"fmt"

	"github.com/tinyverse-web3/tvbase/common/define"
	dmsgKey "github.com/tinyverse-web3/tvbase/dmsg/common/key"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/adapter"
)

type MsgClient struct {
	MsgBase
}

func CreateClient(tvbase define.TvBaseService) (*MsgClient, error) {
	d := &MsgClient{}
	cfg := tvbase.GetConfig().DMsg
	err := d.MsgBase.Init(tvbase, cfg.MaxMsgCount, cfg.KeepMsgDay)
	if err != nil {
		return nil, err
	}
	return d, nil
}

// sdk-common
func (d *MsgClient) Start(pubkey string, getSig dmsgKey.GetSigCallback, isListenMsg bool) error {
	log.Debugf("MsgClient->Init begin")
	ctx := d.TvBase.GetCtx()
	host := d.TvBase.GetHost()

	createPubsubProtocol := adapter.NewCreateMsgPubsubProtocol(ctx, host, d, d, true, pubkey)
	pubsubMsgProtocol := adapter.NewPubsubMsgProtocol(ctx, host, d, d)
	d.RegistPubsubProtocol(pubsubMsgProtocol.Adapter.GetRequestPID(), pubsubMsgProtocol)
	d.RegistPubsubProtocol(pubsubMsgProtocol.Adapter.GetResponsePID(), pubsubMsgProtocol)
	err := d.ProxyPubsubService.Start(pubkey, getSig, createPubsubProtocol, pubsubMsgProtocol, isListenMsg)
	if err != nil {
		return err
	}

	log.Debug("MsgClient->Init end")
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
