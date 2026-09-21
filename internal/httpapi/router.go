package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/platform/observability"
)

// RouterDeps are the dependencies NewRouter wires into routes. Handlers
// are passed in already constructed (against their Service interfaces)
// so the router itself never touches business logic.
type RouterDeps struct {
	Product        *ProductHandler
	Order          *OrderHandler
	Health         *HealthHandler
	Metrics        observability.MetricsRecorder
	MetricsHandler http.Handler // e.g. promhttp.Handler(); nil disables /metrics
	Logger         *slog.Logger
	APIKey         string
}

// NewRouter builds the full HTTP handler: routes plus the middleware
// chain (request id -> panic recovery -> logging -> metrics), with
// api-key auth scoped only to POST /order per the OpenAPI security
// scheme.
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

	return chain(mux,
		RequestID,
		Recover(deps.Logger),
		Logging(deps.Logger),
		Metrics(recorder),
	)
}
