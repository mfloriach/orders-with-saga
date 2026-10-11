package logging

import (
	"context"
	"io"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.opentelemetry.io/otel/trace"
)

type ctxKey string

// RequestIDKey is the Go ctx key carrying the request ID.
const RequestIDKey ctxKey = "request_id"

// RequestIDFrom returns the request ID in ctx, or "".
func RequestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(RequestIDKey).(string)
	return id
}

// Bridge maps the request ID into Go ctx so c.Context() carries it downstream.
func Bridge() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if id, ok := c.Locals("requestid").(string); ok && id != "" {
			c.SetUserContext(context.WithValue(c.UserContext(), RequestIDKey, id))
		}
		return c.Next()
	}
}

// Handler wraps h, injecting request_id and trace_id from ctx into every record.
type Handler struct{ h slog.Handler }

func (h *Handler) Enabled(ctx context.Context, l slog.Level) bool {
	return h.h.Enabled(ctx, l)
}

func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	if id := RequestIDFrom(ctx); id != "" {
		r.AddAttrs(slog.String("request_id", id))
	}
	if tid := trace.SpanFromContext(ctx).SpanContext().TraceID(); tid.IsValid() {
		r.AddAttrs(slog.String("trace_id", tid.String()))
	}
	return h.h.Handle(ctx, r)
}

func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &Handler{h: h.h.WithAttrs(attrs)}
}

func (h *Handler) WithGroup(name string) slog.Handler {
	return &Handler{h: h.h.WithGroup(name)}
}

// Stash stores the start time and trace ID in Locals for error logging.
// Register it after tracing middleware so the span is active.
func Stash() fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Locals("req_start", time.Now())
		if tid := trace.SpanFromContext(c.UserContext()).SpanContext().TraceID(); tid.IsValid() {
			c.Locals("trace_id", tid.String())
		}
		return c.Next()
	}
}

// RequestLogger logs successful responses. Error responses are logged by
// centralized error handling (Fiber runs it after middleware unwinds, when
// the final status is known), so failing requests are skipped here.
func RequestLogger(l *slog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		if err := c.Next(); err != nil {
			return err
		}
		l.InfoContext(c.UserContext(), "request",
			slog.String("method", c.Method()),
			slog.String("path", c.Path()),
			slog.Int("status", c.Response().StatusCode()),
			slog.Duration("latency", time.Since(start)),
		)
		return nil
	}
}

// LogErrorResponse logs a failed request with final status. It rebuilds a
// ctx carrying the stashed trace ID, since tracing middleware already
// restored the pre-span context by the time error handling runs.
func LogErrorResponse(c *fiber.Ctx, status int) {
	ctx := c.UserContext()
	if tid, ok := c.Locals("trace_id").(string); ok && tid != "" {
		if id, err := trace.TraceIDFromHex(tid); err == nil {
			ctx = trace.ContextWithSpanContext(ctx, trace.NewSpanContext(trace.SpanContextConfig{
				TraceID: id, Remote: true,
			}))
		}
	}
	attrs := []any{
		slog.String("method", c.Method()),
		slog.String("path", c.Path()),
		slog.Int("status", status),
	}
	if start, ok := c.Locals("req_start").(time.Time); ok {
		attrs = append(attrs, slog.Duration("latency", time.Since(start)))
	}
	slog.Default().InfoContext(ctx, "request", attrs...)
}

// New returns a JSON logger writing to w.
func New(w io.Writer) *slog.Logger {
	return slog.New(&Handler{h: slog.NewJSONHandler(w, nil)})
}
