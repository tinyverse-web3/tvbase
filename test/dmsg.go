package test

import (
	"time"

	"github.com/tinyverse-web3/tvbase/common/define"
	dmsgKey "github.com/tinyverse-web3/tvbase/dmsg/common/key"
	"github.com/tinyverse-web3/tvbase/dmsg/common/service"
	channelService "github.com/tinyverse-web3/tvbase/dmsg/service/channel"
	customProtocolService "github.com/tinyverse-web3/tvbase/dmsg/service/customProtocol"
	mailboxService "github.com/tinyverse-web3/tvbase/dmsg/service/mailbox"
	msgService "github.com/tinyverse-web3/tvbase/dmsg/service/msg"
)

type DmsgService struct {
	mailboxService        service.MailboxService
	msgService            service.MsgService
	customProtocolService service.CustomProtocolService
	channelService        service.ChannelService
}

func CreateDmsgService(tvbaseService define.TvBaseService) (*DmsgService, error) {
	d := &DmsgService{}
	err := d.Init(tvbaseService)
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (d *DmsgService) Init(tvbase define.TvBaseService) error {
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

func (d *DmsgService) GetMailboxService() service.MailboxService {
	return d.mailboxService
}

func (d *DmsgService) GetMsgService() service.MsgService {
	return d.msgService
}

func (d *DmsgService) GetCustomProtocolService() service.CustomProtocolService {
	return d.customProtocolService
}

func (d *DmsgService) GetChannelService() service.ChannelService {
	return d.channelService
}

func (d *DmsgService) Start(
	enableService bool,
	pubkey string, getSig dmsgKey.GetSigCallback,
	isListenMsg bool,
) error {
	err := d.mailboxService.Start(enableService, pubkey, getSig)
	if err != nil {
		return err
	}
	err = d.mailboxService.CreateMailbox(pubkey, 3*time.Second)
	if err != nil {
		return err
	}
	err = d.msgService.Start(enableService, pubkey, getSig, isListenMsg)
	if err != nil {
		return err
	}
	err = d.channelService.Start(enableService, pubkey, getSig)
	if err != nil {
		return err
	}
	err = d.customProtocolService.Start(enableService, pubkey, getSig)
	if err != nil {
		return err
	}
	return nil
}

func (d *DmsgService) Stop() error {
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
