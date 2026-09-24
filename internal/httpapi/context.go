package httpapi

import (
	"context"
	"log/slog"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/auth"
)

type contextKey int

const (
	requestIDKey contextKey = iota
	loggerKey
	claimsKey
)

// RequestIDFromContext returns the id set by the RequestID middleware, or ""
// if there is none.
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

func withRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

// Falls back to slog.Default() (not nil) when fallback is nil and the
// context has no logger, e.g. a test calling a handler directly.
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

// ClaimsFromContext returns the caller's identity set by the Authenticate
// middleware, and false on routes that don't authenticate.
func ClaimsFromContext(ctx context.Context) (auth.Claims, bool) {
	c, ok := ctx.Value(claimsKey).(auth.Claims)
	return c, ok
}

func withClaims(ctx context.Context, c auth.Claims) context.Context {
	return context.WithValue(ctx, claimsKey, c)
}
