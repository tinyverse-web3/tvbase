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
	mailboxClient         service.MailboxClient
	msgService            service.MsgService
	msgClient             service.MsgClient
	customProtocolService service.CustomProtocolService
	customProtocolClient  service.CustomProtocolClient
	channelService        service.ChannelService
	channelClient         service.ChannelClient
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
	d.mailboxClient, err = mailboxService.CreateClient(tvbase)
	if err != nil {
		return err
	}
	d.msgService, err = msgService.CreateService(tvbase)
	if err != nil {
		return err
	}
	d.msgClient, err = msgService.CreateClient(tvbase)
	if err != nil {
		return err
	}
	d.channelService, err = channelService.CreateService(tvbase)
	if err != nil {
		return err
	}
	d.channelClient, err = channelService.CreateClient(tvbase)
	if err != nil {
		return err
	}
	d.customProtocolService, err = customProtocolService.CreateService(tvbase)
	if err != nil {
		return err
	}
	d.customProtocolClient, err = customProtocolService.CreateClient(tvbase)
	if err != nil {
		return err
	}
	return nil
}

func (d *DmsgService) GetMailboxService() service.MailboxService {
	return d.mailboxService
}

func (d *DmsgService) GetMailboxClient() service.MailboxClient {
	return d.mailboxClient
}

func (d *DmsgService) GetMsgService() service.MsgService {
	return d.msgService
}

func (d *DmsgService) GetMsgClient() service.MsgClient {
	return d.msgClient
}

func (d *DmsgService) GetCustomProtocolService() service.CustomProtocolService {
	return d.customProtocolService
}

func (d *DmsgService) GetCustomProtocolClient() service.CustomProtocolClient {
	return d.customProtocolClient
}

func (d *DmsgService) GetChannelService() service.ChannelService {
	return d.channelService
}

func (d *DmsgService) GetChannelClient() service.ChannelClient {
	return d.channelClient
}

func (d *DmsgService) Start(
	pubkey string,
	getSig dmsgKey.GetSigCallback,
	timeout time.Duration,
	isListenMsg bool,
) error {

	err := d.mailboxService.Start(pubkey, getSig)
	if err != nil {
		return err
	}
	err = d.mailboxClient.Start(pubkey, getSig)
	if err != nil {
		return err
	}
	err = d.msgService.Start(pubkey, getSig, isListenMsg)
	if err != nil {
		return err
	}
	err = d.msgClient.Start(pubkey, getSig, isListenMsg)
	if err != nil {
		return err
	}
	err = d.channelService.Start(pubkey, getSig)
	if err != nil {
		return err
	}
	err = d.channelClient.Start(pubkey, getSig)
	if err != nil {
		return err
	}
	err = d.customProtocolService.Start(pubkey, getSig)
	if err != nil {
		return err
	}
	err = d.customProtocolClient.Start(pubkey, getSig)
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
