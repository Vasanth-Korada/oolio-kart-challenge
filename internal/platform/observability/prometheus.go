package observability

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Prometheus is the production MetricsRecorder, exposing counters and
// histograms in the standard /metrics text format.
type Prometheus struct {
	requestsTotal   *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
}

// NewPrometheus registers the service's metrics with the default
// Prometheus registry and returns a recorder for the HTTP middleware.
func NewPrometheus() *Prometheus {
	return &Prometheus{
		requestsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests, by method, route, and status code.",
		}, []string{"method", "route", "status"}),
		requestDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds, by method and route.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "route"}),
	}
}

func (p *Prometheus) ObserveRequest(method, route string, status int, duration time.Duration) {
	statusLabel := strconv.Itoa(status)
	p.requestsTotal.WithLabelValues(method, route, statusLabel).Inc()
	p.requestDuration.WithLabelValues(method, route).Observe(duration.Seconds())
}

// Handler serves the /metrics scrape endpoint.
func (p *Prometheus) Handler() http.Handler {
	return promhttp.Handler()
}

var _ MetricsRecorder = (*Prometheus)(nil)
