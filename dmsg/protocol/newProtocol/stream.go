package newProtocol

import (
	"context"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/basic"
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
) *basic.CreatePubsubSProtocol {
	p := &basic.CreatePubsubSProtocol{}
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
) *basic.CreatePubsubSProtocol {
	p := &basic.CreatePubsubSProtocol{}
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
) *basic.MailboxSProtocol {
	p := &basic.MailboxSProtocol{}
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
) *basic.CustomSProtocol {
	protocol := &basic.CustomSProtocol{}
	protocol.Host = host
	protocol.Ctx = ctx
	protocol.Callback = callback
	protocol.Service = service
	protocol.Adapter = adapter
	protocol.Host.SetStreamHandler(adapter.GetStreamResponsePID(), protocol.ResponseHandler)
	if enableRequest {
		protocol.Host.SetStreamHandler(adapter.GetStreamRequestPID(), protocol.RequestHandler)
	}
	go protocol.TickCleanRequest()
	return protocol
}
