package basic

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/common"
)

func NewCreateMsgPubsubSProtocol(
	ctx context.Context,
	host host.Host,
	callback common.CreatePubsubSpCallback,
	service common.DmsgService,
	adapter common.SpAdapter,
	enableRequest bool,
	pubkey string,
) *CreatePubsubSProtocol {
	p := &CreatePubsubSProtocol{}
	p.Host = host
	p.Ctx = ctx
	p.Callback = callback
	p.Service = service
	p.Adapter = adapter
	respPid := protocol.ID(string(adapter.GetStreamResponsePID()) + "/" + pubkey)
	p.Host.SetStreamHandler(respPid, p.ResponseHandler)
	if enableRequest {
		p.Host.SetStreamHandler(adapter.GetStreamRequestPID(), p.RequestHandler)
	}
	go p.TickCleanRequest()
	return p
}

func NewCreateChannelSProtocol(
	ctx context.Context,
	host host.Host,
	callback common.CreatePubsubSpCallback,
	service common.DmsgService,
	adapter common.SpAdapter,
	enableRequest bool,
	pubkey string,
) *CreatePubsubSProtocol {
	p := &CreatePubsubSProtocol{}
	p.Host = host
	p.Ctx = ctx
	p.Callback = callback
	p.Service = service
	p.Adapter = adapter
	respPid := protocol.ID(string(adapter.GetStreamResponsePID()) + "/" + pubkey)
	p.Host.SetStreamHandler(respPid, p.ResponseHandler)
	if enableRequest {
		p.Host.SetStreamHandler(adapter.GetStreamRequestPID(), p.RequestHandler)
	}
	go p.TickCleanRequest()
	return p
}

func NewMailboxSProtocol(
	ctx context.Context,
	host host.Host,
	callback common.MailboxSpCallback,
	service common.DmsgService,
	adapter common.SpAdapter,
	enableRequest bool,
	pubkey string,
) *MailboxSProtocol {
	p := &MailboxSProtocol{}
	p.Host = host
	p.Ctx = ctx
	p.Callback = callback
	p.Service = service
	p.Adapter = adapter
	respPid := protocol.ID(string(adapter.GetStreamResponsePID()) + "/" + pubkey)
	p.Host.SetStreamHandler(respPid, p.ResponseHandler)
	if enableRequest {
		p.Host.SetStreamHandler(adapter.GetStreamRequestPID(), p.RequestHandler)
	}
	go p.TickCleanRequest()
	return p
}

func NewCustomSProtocol(
	ctx context.Context,
	host host.Host,
	callback common.CustomSpCallback,
	service common.DmsgService,
	adapter common.SpAdapter,
	enableRequest bool,
	pubkey string,
) *CustomSProtocol {
	p := &CustomSProtocol{}
	p.Host = host
	p.Ctx = ctx
	p.Callback = callback
	p.Service = service
	p.Adapter = adapter
	respPid := protocol.ID(string(adapter.GetStreamResponsePID()) + "/" + pubkey)
	fmt.Printf("SetStreamHandler : respPid = %s\n", respPid)
	p.Host.SetStreamHandler(respPid, p.ResponseHandler)
	if enableRequest {
		p.Host.SetStreamHandler(adapter.GetStreamRequestPID(), p.RequestHandler)
	}
	go p.TickCleanRequest()
	return p
}
