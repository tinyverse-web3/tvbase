package tvbase

import (
	"context"
	"fmt"

	kaddht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/tinyverse-web3/tvbase/common/config"
	"github.com/tinyverse-web3/tvbase/common/define"
	tvLog "github.com/tinyverse-web3/tvbase/common/log"
	tvProtocol "github.com/tinyverse-web3/tvbase/common/protocol"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func (m *TvBase) GetNodeInfoService() *tvProtocol.NodeInfoService {
	return m.nodeInfoService
}

func (m *TvBase) GetDht() *kaddht.IpfsDHT {
	return m.dht
}

func (m *TvBase) GetCtx() context.Context {
	return m.ctx
}

func (m *TvBase) GetHost() host.Host {
	return m.host
}

func (m *TvBase) GetRootPath() string {
	return m.rootPath
}

func (m *TvBase) GetConfig() *config.TvbaseConfig {
	return m.cfg
}

func (m *TvBase) GetTraceSpan() trace.Span {
	return m.TracerSpan
}

func (m *TvBase) TraceSpan(componentName string, spanName string, options ...any) error {
	var key string
	var value string
	var ok bool
	var ctx context.Context
	var span trace.Span
	if len(options) >= 2 {
		key, ok = options[0].(string)
		if !ok {
			tvLog.Logger.Errorf("tvBase->TraceSpan: options[0](key) is not string")
			return fmt.Errorf("tvBase->TraceSpan: options[0](key) is not string")
		}
		value, ok = options[1].(string)
		if !ok {
			tvLog.Logger.Errorf("tvBase->TraceSpan: options[1](value) is not string")
			return fmt.Errorf("tvBase->TraceSpan: options[1](value) is not string")
		}
		ctx, span = Span(m.ctx, componentName, spanName, trace.WithAttributes(attribute.String(key, value)))
	} else {
		ctx, span = Span(m.ctx, componentName, spanName)
	}

	defer span.End()

	if len(options) >= 3 {
		traceSpanCallback, ok := options[1].(define.TraceSpanCallback)
		if !ok {
			tvLog.Logger.Errorf("tvBase->TraceSpan: options[2](traceSpanCallback) is not TraceSpanCallback")
			return fmt.Errorf("tvBase->TraceSpan: options[2](traceSpanCallback) is not TraceSpanCallback")
		}
		traceSpanCallback(ctx)
	}

	return nil
}

func (m *TvBase) SetTracerStatus(err error) {
	if m.TracerSpan == nil {
		tvLog.Logger.Warnf("tvBase->SetTracerStatus: span is nil")
		return
	}
	m.TracerSpan.SetStatus(codes.Error, err.Error())
}

func (m *TvBase) GetAvailableServicePeerList(key string) ([]peer.ID, error) {
	return m.getAvailablePeerList(key, config.ServiceMode)
}

func (m *TvBase) GetAvailableLightPeerList(key string) ([]peer.ID, error) {
	return m.getAvailablePeerList(key, config.LightMode)
}
