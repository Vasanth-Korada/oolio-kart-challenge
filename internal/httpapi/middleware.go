package httpapi

import (
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/auth"
	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/platform/idgen"
)

// Middleware wraps an http.Handler with extra behaviour.
type Middleware func(http.Handler) http.Handler

func chain(handler http.Handler, middlewares ...Middleware) http.Handler {
	for position := len(middlewares) - 1; position >= 0; position-- {
		handler = middlewares[position](handler)
	}
	return handler
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

// WriteHeader records the status code for the access log.
func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// RequestID reuses the client's X-Request-Id or generates a UUID, echoes it
// in the response, and stores it in the request context.
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

// Logging writes one JSON access log line per request (method, path, status,
// duration) and puts a request-scoped logger in the context.
func Logging(base *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			reqLogger := base.With(slog.String("request_id", RequestIDFromContext(r.Context())))
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(rec, r.WithContext(withLogger(r.Context(), reqLogger)))

			// duration_ms, not slog.Duration: the JSON handler renders
			// slog.Duration as raw nanoseconds, hard to eyeball.
			reqLogger.Info("http request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", rec.status),
				slog.Float64("duration_ms", float64(time.Since(start).Microseconds())/1000.0),
			)
		})
	}
}

// Recover turns a panic in a handler into a 500 response and an error log,
// instead of dropping the connection.
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

// CORS sets the CORS headers for allowedOrigin ("*" allows any origin) and
// answers preflight OPTIONS requests with 204.
func CORS(allowedOrigin string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				if allowedOrigin == "*" {
					w.Header().Set("Access-Control-Allow-Origin", "*")
				} else if origin == allowedOrigin {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Vary", "Origin")
				}
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, api_key")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Authentication methods, used as the AuthMetrics method label.
const (
	authMethodPassword = "password"
	authMethodBearer   = "bearer"
	authMethodAPIKey   = "api_key"
)

// apiKeySubject is the subject recorded for callers using the static
// api_key. The spec grants that key the create_order scope.
const apiKeySubject = "api_key"

// Authenticate identifies the caller and stores auth.Claims in the context.
// It accepts either credential, Bearer first when both are sent:
//
//   - Authorization: Bearer <JWT>, checked by verifier. A malformed header or
//     a bad, expired or tampered token is 401.
//   - api_key: <key>, compared with apiKey in constant time. A wrong key is
//     403, as before JWT support; it grants the create_order scope.
//
// No credentials at all is 401. Every 401 carries WWW-Authenticate: Bearer.
func Authenticate(verifier auth.TokenVerifier, apiKey string, recorder AuthMetrics) Middleware {
	if recorder == nil {
		recorder = noAuthMetrics{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if header := r.Header.Get("Authorization"); header != "" {
				claims, err := verifyBearer(verifier, header)
				recorder.AuthAttempt(authMethodBearer, err == nil)
				if err != nil {
					writeUnauthorized(w, `Bearer error="invalid_token"`, "invalid or expired token")
					return
				}
				next.ServeHTTP(w, r.WithContext(withClaims(r.Context(), claims)))
				return
			}

			if key := r.Header.Get("api_key"); key != "" {
				ok := subtle.ConstantTimeCompare([]byte(key), []byte(apiKey)) == 1
				recorder.AuthAttempt(authMethodAPIKey, ok)
				if !ok {
					WriteError(w, http.StatusForbidden, "forbidden", "invalid api_key")
					return
				}
				claims := auth.Claims{Subject: apiKeySubject, Scopes: []string{auth.ScopeCreateOrder}}
				next.ServeHTTP(w, r.WithContext(withClaims(r.Context(), claims)))
				return
			}

			writeUnauthorized(w, "Bearer", "missing credentials: send an Authorization Bearer token or the api_key header")
		})
	}
}

// RequireScope lets the request through only if the authenticated caller
// has scope: 403 otherwise, 401 if Authenticate did not run first.
func RequireScope(scope string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := ClaimsFromContext(r.Context())
			if !ok {
				writeUnauthorized(w, "Bearer", "missing credentials")
				return
			}
			if !claims.HasScope(scope) {
				w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer error="insufficient_scope", scope=%q`, scope))
				WriteError(w, http.StatusForbidden, "forbidden", "token lacks the "+scope+" scope")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// verifyBearer parses "Bearer <token>" (scheme is case-insensitive, per
// RFC 6750) and verifies the token.
func verifyBearer(verifier auth.TokenVerifier, header string) (auth.Claims, error) {
	scheme, token, ok := strings.Cut(header, " ")
	token = strings.TrimSpace(token)
	if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" {
		return auth.Claims{}, auth.ErrInvalidToken
	}
	return verifier.Verify(token)
}

// writeUnauthorized writes a 401 with the WWW-Authenticate challenge RFC
// 6750 requires.
func writeUnauthorized(w http.ResponseWriter, challenge, message string) {
	w.Header().Set("WWW-Authenticate", challenge)
	WriteError(w, http.StatusUnauthorized, "unauthorized", message)
}
