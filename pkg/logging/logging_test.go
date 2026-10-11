package logging

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func TestRecordCarriesIDs(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf)
	tid := trace.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	sid := trace.SpanID{1, 2, 3, 4, 5, 6, 7, 8}
	ctx := context.WithValue(context.Background(), RequestIDKey, "req-1")
	ctx = trace.ContextWithSpanContext(ctx, trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: tid, SpanID: sid, TraceFlags: trace.FlagsSampled, Remote: true,
	}))
	l.InfoContext(ctx, "hello")
	out := buf.String()
	for _, want := range []string{`"request_id":"req-1"`, `"trace_id":"` + tid.String() + `"`, `"msg":"hello"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %s in %s", want, out)
		}
	}
}

func TestRecordWithoutIDs(t *testing.T) {
	var buf bytes.Buffer
	New(&buf).InfoContext(context.Background(), "hello")
	if out := buf.String(); strings.Contains(out, "request_id") || strings.Contains(out, "trace_id") {
		t.Fatalf("expected no IDs in %s", out)
	}
}
