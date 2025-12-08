package telemetry

import (
	"context"
	"log/slog"
	"os"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/trace"
)

// SetupLogger configures the global slog logger with a hybrid handler (OTLP + Stdout)
func SetupLogger(loggerProvider *log.LoggerProvider, serviceName string) {
	// 1. OTLP Handler (sends to collector)
	otlpHandler := otelslog.NewHandler(serviceName,
		otelslog.WithLoggerProvider(loggerProvider),
	)

	// 2. Stdout Handler (prints JSON to console)
	// We wrap it in TraceHandler to add trace_id/span_id to console logs
	stdoutHandler := NewTraceHandler(slog.NewJSONHandler(os.Stdout, nil))

	// 3. Combine them (Fanout)
	multiHandler := FanoutHandler{
		handlers: []slog.Handler{otlpHandler, stdoutHandler},
	}

	// 4. Set as global logger
	logger := slog.New(multiHandler)
	slog.SetDefault(logger)
}

// FanoutHandler duplicates log records to multiple handlers
type FanoutHandler struct {
	handlers []slog.Handler
}

func (h FanoutHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h FanoutHandler) Handle(ctx context.Context, r slog.Record) error {
	var firstErr error
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, r.Level) {
			if err := handler.Handle(ctx, r); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func (h FanoutHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		handlers[i] = handler.WithAttrs(attrs)
	}
	return FanoutHandler{handlers: handlers}
}

func (h FanoutHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		handlers[i] = handler.WithGroup(name)
	}
	return FanoutHandler{handlers: handlers}
}

// TraceHandler adds trace_id and span_id to the log record from the context
type TraceHandler struct {
	slog.Handler
}

func NewTraceHandler(h slog.Handler) *TraceHandler {
	return &TraceHandler{Handler: h}
}

func (h *TraceHandler) Handle(ctx context.Context, r slog.Record) error {
	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		// Add trace context attributes to the record
		// We create a new record with the added attributes to avoid mutating the original
		// However, slog.Record is passed by value, but its internal storage might be shared.
		// The safest way is to use WithAttrs if we were returning a handler, but here we are handling a record.
		// slog.Record.AddAttrs appends to the record.
		
		// Note: We clone the record implicitly by passing it by value, but we need to be careful.
		// Actually, r.AddAttrs modifies the record 'r' which is a copy.
		r.AddAttrs(
			slog.String("trace_id", span.SpanContext().TraceID().String()),
			slog.String("span_id", span.SpanContext().SpanID().String()),
		)
	}
	return h.Handler.Handle(ctx, r)
}
