package httpapi

import (
	"log/slog"
	"net/http"
)

// RouterDeps holds everything NewRouter wires together.
type RouterDeps struct {
	Product    *ProductHandler
	Order      *OrderHandler
	Health     *HealthHandler
	Logger     *slog.Logger
	APIKey     string
	CORSOrigin string // "" disables CORS headers entirely
}

// NewRouter registers all routes and wraps them in the global middleware,
// outermost first: RequestID, Recover, Logging, then CORS when enabled.
// APIKeyAuth guards POST /order only.
func NewRouter(deps RouterDeps) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /product", deps.Product.List)
	mux.HandleFunc("GET /product/{id}", deps.Product.Get)

	orderRoute := chain(http.HandlerFunc(deps.Order.Create), APIKeyAuth(deps.APIKey))
	mux.Handle("POST /order", orderRoute)

	mux.HandleFunc("GET /healthz", deps.Health.Live)
	mux.HandleFunc("GET /readyz", deps.Health.Ready)

	mws := []Middleware{
		RequestID,
		Recover(deps.Logger),
		Logging(deps.Logger),
	}
	if deps.CORSOrigin != "" {
		mws = append(mws, CORS(deps.CORSOrigin))
	}
	return chain(mux, mws...)
}
