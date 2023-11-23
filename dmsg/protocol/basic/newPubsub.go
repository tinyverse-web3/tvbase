package basic

import (
	"context"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/common"
)

func NewQueryPeerProtocol(
	ctx context.Context,
	host host.Host,
	callback common.QueryPeerCallback,
	dmsg common.DmsgService,
	adapter common.PpAdapter) *QueryPeerProtocol {
	ret := &QueryPeerProtocol{}
	ret.Ctx = ctx
	ret.Host = host
	ret.Callback = callback
	ret.Service = dmsg
	ret.Adapter = adapter
	go ret.TickCleanRequest()
	return ret
}

func NewPubsubMsgProtocol(
	ctx context.Context,
	host host.Host,
	callback common.PubsubMsgCallback,
	dmsg common.DmsgService,
	adapter common.PpAdapter) *PubsubMsgProtocol {
	ret := &PubsubMsgProtocol{}
	ret.Ctx = ctx
	ret.Host = host
	ret.Callback = callback
	ret.Service = dmsg
	ret.Adapter = adapter
	go ret.TickCleanRequest()
	return ret
}

func NewMailboxPProtocol(
	ctx context.Context,
	host host.Host,
	callback common.MailboxPpCallback,
	dmsg common.DmsgService,
	adapter common.PpAdapter) *MailboxPProtocol {
	ret := &MailboxPProtocol{}
	ret.Ctx = ctx
	ret.Host = host
	ret.Callback = callback
	ret.Service = dmsg
	ret.Adapter = adapter
	go ret.TickCleanRequest()
	return ret
}
