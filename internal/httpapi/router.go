package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/platform/observability"
)

// RouterDeps are the dependencies NewRouter wires into routes.
type RouterDeps struct {
	Product        *ProductHandler
	Order          *OrderHandler
	Health         *HealthHandler
	Metrics        observability.MetricsRecorder
	MetricsHandler http.Handler // e.g. promhttp.Handler(); nil disables /metrics
	Logger         *slog.Logger
	APIKey         string
	CORSOrigin     string // "" disables CORS headers entirely
}

// NewRouter builds the full HTTP handler: routes plus the middleware
// chain, with api-key auth scoped only to POST /order.
func NewRouter(deps RouterDeps) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /product", deps.Product.List)
	mux.HandleFunc("GET /product/{id}", deps.Product.Get)

	orderRoute := chain(http.HandlerFunc(deps.Order.Create), APIKeyAuth(deps.APIKey))
	mux.Handle("POST /order", orderRoute)

	mux.HandleFunc("GET /healthz", deps.Health.Live)
	mux.HandleFunc("GET /readyz", deps.Health.Ready)

	if deps.MetricsHandler != nil {
		mux.Handle("GET /metrics", deps.MetricsHandler)
	}

	recorder := deps.Metrics
	if recorder == nil {
		recorder = observability.NoOp{}
	}

	mws := []Middleware{
		RequestID,
		Recover(deps.Logger),
		Logging(deps.Logger),
		Metrics(recorder),
	}
	if deps.CORSOrigin != "" {
		mws = append(mws, CORS(deps.CORSOrigin))
	}
	return chain(mux, mws...)
}
