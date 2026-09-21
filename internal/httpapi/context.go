package httpapi

import (
	"context"
	"log/slog"
)

type contextKey int

const (
	requestIDKey contextKey = iota
	loggerKey
)

// RequestIDFromContext returns the request id set by the RequestID
// middleware, or "" if none is present (e.g. in a unit test that calls a
// handler directly).
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

func withRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

// LoggerFromContext returns the request-scoped logger (already tagged
// with request_id) set by the Logging middleware, falling back to the
// given logger, or slog.Default() if fallback is nil, so callers never
// get a nil logger back (nil is only ever hit in a test that invokes a
// handler directly, bypassing the Logging middleware).
func LoggerFromContext(ctx context.Context, fallback *slog.Logger) *slog.Logger {
	if l, ok := ctx.Value(loggerKey).(*slog.Logger); ok && l != nil {
		return l
	}
	if fallback != nil {
		return fallback
	}
	return slog.Default()
}

func withLogger(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, l)
}
