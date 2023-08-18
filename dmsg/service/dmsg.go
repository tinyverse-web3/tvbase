package service

import (
	tvbaseCommon "github.com/tinyverse-web3/tvbase/common"
	dmsgKey "github.com/tinyverse-web3/tvbase/dmsg/common/key"
	dmsgCommonPubsub "github.com/tinyverse-web3/tvbase/dmsg/common/pubsub"
	channelService "github.com/tinyverse-web3/tvbase/dmsg/service/channel"
	customProtocolService "github.com/tinyverse-web3/tvbase/dmsg/service/customProtocol"
	mailboxService "github.com/tinyverse-web3/tvbase/dmsg/service/mailbox"
	msgService "github.com/tinyverse-web3/tvbase/dmsg/service/msg"

	service "github.com/tinyverse-web3/tvbase/dmsg/common/service"
)

type Dmsg struct {
	mailboxService        service.MailboxService
	msgService            service.MsgService
	customProtocolService service.CustomProtocolService
	channelService        service.ChannelService
}

func CreateService(tvbaseService tvbaseCommon.TvBaseService) (*Dmsg, error) {
	dmsgCommonPubsub.NewPubsubMgr(tvbaseService.GetCtx(), tvbaseService.GetHost())
	d := &Dmsg{}
	err := d.Init(tvbaseService)
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (d *Dmsg) Init(tvbase tvbaseCommon.TvBaseService) error {
	var err error
	d.mailboxService, err = mailboxService.CreateService(tvbase)
	if err != nil {
		return err
	}
	d.msgService, err = msgService.CreateService(tvbase)
	if err != nil {
		return err
	}
	d.channelService, err = channelService.CreateService(tvbase)
	if err != nil {
		return err
	}
	d.customProtocolService, err = customProtocolService.CreateService(tvbase)
	if err != nil {
		return err
	}
	return nil
}

func (d *Dmsg) GetMailboxService() service.MailboxService {
	return d.mailboxService
}

func (d *Dmsg) GetMsgService() service.MsgService {
	return d.msgService
}

func (d *Dmsg) GetCustomProtocolService() service.CustomProtocolService {
	return d.customProtocolService
}

func (d *Dmsg) GetChannelService() service.ChannelService {
	return d.channelService
}

func (d *Dmsg) Start(
	enableService bool,
	pubkeyData []byte,
	getSig dmsgKey.GetSigCallback) error {
	err := d.mailboxService.Start(enableService, pubkeyData, getSig)
	if err != nil {
		return err
	}
	err = d.msgService.Start(enableService, pubkeyData, getSig)
	if err != nil {
		return err
	}
	err = d.channelService.Start(enableService, pubkeyData, getSig)
	if err != nil {
		return err
	}
	err = d.customProtocolService.Start(enableService, pubkeyData, getSig)
	if err != nil {
		return err
	}
	return nil
}

func (d *Dmsg) Stop() error {
	err := d.mailboxService.Stop()
	if err != nil {
		return err
	}
	err = d.msgService.Stop()
	if err != nil {
		return err
	}
	err = d.channelService.Stop()
	if err != nil {
		return err
	}
	err = d.customProtocolService.Stop()
	if err != nil {
		return err
	}
	return nil
}
