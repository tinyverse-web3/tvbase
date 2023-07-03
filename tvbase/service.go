package tvbase

import (
	"context"
	"fmt"

	kaddht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	tvCommon "github.com/tinyverse-web3/tvbase/common"
	tvConfig "github.com/tinyverse-web3/tvbase/common/config"
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

func (m *TvBase) GetConfig() *tvConfig.NodeConfig {
	return m.nodeCfg
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
			tvLog.Logger.Errorf("Infrasture->TraceSpan: options[0](key) is not string")
			return fmt.Errorf("Infrasture->TraceSpan: options[0](key) is not string")
		}
		value, ok = options[1].(string)
		if !ok {
			tvLog.Logger.Errorf("Infrasture->TraceSpan: options[1](value) is not string")
			return fmt.Errorf("Infrasture->TraceSpan: options[1](value) is not string")
		}
		ctx, span = Span(m.ctx, componentName, spanName, trace.WithAttributes(attribute.String(key, value)))
	} else {
		ctx, span = Span(m.ctx, componentName, spanName)
	}

	defer span.End()

	if len(options) >= 3 {
		traceSpanCallback, ok := options[1].(tvCommon.TraceSpanCallback)
		if !ok {
			tvLog.Logger.Errorf("Infrasture->TraceSpan: options[2](traceSpanCallback) is not TraceSpanCallback")
			return fmt.Errorf("Infrasture->TraceSpan: options[2](traceSpanCallback) is not TraceSpanCallback")
		}
		traceSpanCallback(ctx)
	}

	return nil
}

func (m *TvBase) SetTracerStatus(err error) {
	if m.TracerSpan == nil {
		tvLog.Logger.Warnf("Infrasture->SetTracerStatus: span is nil")
		return
	}
	m.TracerSpan.SetStatus(codes.Error, err.Error())
}

func (m *TvBase) GetAvailableServicePeerList() ([]peer.ID, error) {
	return m.getAvailablePeerList(tvConfig.FullMode)
}

func (m *TvBase) GetAvailableLightPeerList() ([]peer.ID, error) {
	return m.getAvailablePeerList(tvConfig.LightMode)
}
