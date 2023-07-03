package tvbase

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/ipfs/boxo/tracing"
	logging "github.com/ipfs/go-log"
	tvCommon "github.com/tinyverse-web3/tvbase/common"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/resource"
	sdkTrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

// tracing
// shutdownTracerProvider adds a shutdown method for tracer providers.
//
// Note that this doesn't directly use the provided TracerProvider interface
// to avoid build breaking go-ipfs if new methods are added to it.
type shutdownTracerProvider interface {
	Tracer(instrumentationName string, opts ...trace.TracerOption) trace.Tracer
	Shutdown(ctx context.Context) error
}

// noopShutdownTracerProvider adds a no-op Shutdown method to a TracerProvider.
type noopShutdownTracerProvider struct{ trace.TracerProvider }

func (n *noopShutdownTracerProvider) Shutdown(ctx context.Context) error { return nil }

// NewTracerProvider creates and configures a TracerProvider.
func NewTracerProvider(ctx context.Context) (shutdownTracerProvider, error) {
	exporters, err := tracing.NewSpanExporters(ctx)
	if err != nil {
		return nil, err
	}
	if len(exporters) == 0 {
		return &noopShutdownTracerProvider{TracerProvider: trace.NewNoopTracerProvider()}, nil
	}

	options := []sdkTrace.TracerProviderOption{}

	for _, exporter := range exporters {
		options = append(options, sdkTrace.WithBatcher(exporter))
	}

	r, err := resource.Merge(
		resource.Default(),
		resource.NewSchemaless(
			semconv.ServiceNameKey.String("tinverseInfrasture"),
			semconv.ServiceVersionKey.String(tvCommon.CurrentVersionNumber),
		),
	)
	if err != nil {
		return nil, err
	}
	options = append(options, sdkTrace.WithResource(r))

	return sdkTrace.NewTracerProvider(options...), nil
}

// Span starts a new span using the standard IPFS tracing conventions.
func Span(ctx context.Context, componentName string, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return otel.Tracer("Kubo").Start(ctx, fmt.Sprintf("%s.%s", componentName, spanName), opts...)
}

func newUUID(key string) logging.Metadata {
	ids := "#UUID-ERROR#"
	if id, err := uuid.NewRandom(); err == nil {
		ids = id.String()
	}
	return logging.Metadata{
		key: ids,
	}
}
