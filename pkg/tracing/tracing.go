// Package tracing initializes OpenTelemetry with no exporter: spans are
// created (so trace IDs exist for log correlation) and dropped locally.
// Adding an exporter later is config-only.
package tracing

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func Init() *sdktrace.TracerProvider {
	tp := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	return tp
}
