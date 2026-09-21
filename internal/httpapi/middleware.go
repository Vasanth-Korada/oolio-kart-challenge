package httpapi

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
	"time"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/platform/idgen"
	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/platform/observability"
)

// Middleware is the standard net/http decorator shape used throughout
// this package, composed with chain().
type Middleware func(http.Handler) http.Handler

func chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// statusRecorder captures the status code written by the wrapped
// handler so Logging/metrics middleware can observe it — net/http's
// ResponseWriter doesn't expose what was already written.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// RequestID assigns a request id (from X-Request-Id if the caller
// supplied one, else generated) to the request context and response
// header, so it can be correlated across logs, traces, and client
// support tickets.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if id == "" {
			id = idgen.NewUUID()
		}
		w.Header().Set("X-Request-Id", id)
		next.ServeHTTP(w, r.WithContext(withRequestID(r.Context(), id)))
	})
}

// Logging logs one structured line per request (method, path, status,
// duration, request id) and attaches a request-scoped logger to the
// context for handlers/services to use, so every log line downstream of
// a request carries the same request_id without threading it manually.
func Logging(base *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			reqLogger := base.With(slog.String("request_id", RequestIDFromContext(r.Context())))
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(rec, r.WithContext(withLogger(r.Context(), reqLogger)))

			reqLogger.Info("http request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", rec.status),
				slog.Duration("duration", time.Since(start)),
			)
		})
	}
}

// Metrics records per-request counters/histograms via the given
// recorder. Kept behind the observability.MetricsRecorder interface so
// the backend (Prometheus today) can change without touching this
// middleware or the handlers.
func Metrics(recorder observability.MetricsRecorder) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(rec, r)

			route := r.Pattern
			if route == "" {
				route = r.URL.Path
			}
			recorder.ObserveRequest(r.Method, route, rec.status, time.Since(start))
		})
	}
}

// Recover turns a panic in any downstream handler into a 500 response
// instead of crashing the process, logging the panic with a stack trace
// for diagnosis.
func Recover(base *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					LoggerFromContext(r.Context(), base).Error("panic recovered",
						slog.Any("panic", rec),
					)
					WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// APIKeyAuth enforces the OpenAPI spec's api_key security scheme:
// missing header -> 401, present but wrong -> 403. The comparison is
// constant-time so response timing can't be used to brute-force the key.
func APIKeyAuth(expected string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got := r.Header.Get("api_key")
			if got == "" {
				WriteError(w, http.StatusUnauthorized, "unauthorized", "missing api_key header")
				return
			}
			if subtle.ConstantTimeCompare([]byte(got), []byte(expected)) != 1 {
				WriteError(w, http.StatusForbidden, "forbidden", "invalid api_key")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
