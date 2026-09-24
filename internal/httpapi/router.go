package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/auth"
)

// metricsRoute is the ServeMux pattern for the Prometheus endpoint.
const metricsRoute = "GET /metrics"

// RouterDeps holds everything NewRouter wires together.
type RouterDeps struct {
	Product    *ProductHandler
	Order      *OrderHandler
	Health     *HealthHandler
	Logger     *slog.Logger
	CORSOrigin string // "" disables CORS headers entirely

	// Auth serves POST /auth/token. POST /order accepts a Bearer token that
	// TokenVerifier accepts, or the static APIKey.
	Auth          *AuthHandler
	TokenVerifier auth.TokenVerifier
	APIKey        string

	// Metrics records HTTP metrics and MetricsHandler serves them on
	// GET /metrics. Both are optional; leave them nil to disable metrics.
	Metrics        HTTPMetrics
	MetricsHandler http.Handler
	// AuthMetrics counts authentication attempts; optional.
	AuthMetrics AuthMetrics
}

// NewRouter registers all routes and wraps them in the global middleware,
// outermost first: RequestID, Metrics (when set), Recover, Logging, then CORS
// when enabled. Metrics sits outside Recover so a recovered panic is still
// counted as a 500. POST /order requires Authenticate plus the create_order
// scope; the other routes are public.
func NewRouter(deps RouterDeps) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /product", deps.Product.List)
	mux.HandleFunc("GET /product/{id}", deps.Product.Get)

	mux.HandleFunc("POST /auth/token", deps.Auth.Token)

	orderRoute := chain(http.HandlerFunc(deps.Order.Create),
		Authenticate(deps.TokenVerifier, deps.APIKey, deps.AuthMetrics),
		RequireScope(auth.ScopeCreateOrder),
	)
	mux.Handle("POST /order", orderRoute)

	mux.HandleFunc("GET /healthz", deps.Health.Live)
	mux.HandleFunc("GET /readyz", deps.Health.Ready)

	if deps.MetricsHandler != nil {
		mux.Handle(metricsRoute, deps.MetricsHandler)
	}

	mws := []Middleware{RequestID}
	if deps.Metrics != nil {
		routeOf := func(r *http.Request) string {
			_, pattern := mux.Handler(r)
			return pattern
		}
		mws = append(mws, Metrics(deps.Metrics, routeOf, metricsRoute))
	}
	mws = append(mws, Recover(deps.Logger), Logging(deps.Logger))
	if deps.CORSOrigin != "" {
		mws = append(mws, CORS(deps.CORSOrigin))
	}
	return chain(mux, mws...)
}
