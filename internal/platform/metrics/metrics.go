// Package metrics is the Prometheus implementation of the app's metrics:
// HTTP request rate, errors and latency, order and coupon business events,
// authentication attempts, and Go runtime and process metrics. Other packages
// depend on their own small interfaces (httpapi.HTTPMetrics,
// httpapi.AuthMetrics, order.Recorder); this package
// satisfies them, so only it imports the Prometheus client.
package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const namespace = "oolio"

// Metrics holds every collector on its own registry (not the global one),
// so tests can create independent instances.
type Metrics struct {
	registry *prometheus.Registry

	httpRequests *prometheus.CounterVec
	httpDuration *prometheus.HistogramVec
	httpInFlight prometheus.Gauge

	ordersPlaced    *prometheus.CounterVec
	orderRejections *prometheus.CounterVec
	orderAmount     prometheus.Histogram
	couponChecks    *prometheus.CounterVec
	authAttempts    *prometheus.CounterVec

	couponIndexCodes prometheus.Gauge
	storage          *prometheus.GaugeVec
}

// New creates the collectors and registers them, plus the standard Go
// runtime and process collectors.
func New() *Metrics {
	m := &Metrics{
		registry: prometheus.NewRegistry(),
		httpRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace, Subsystem: "http", Name: "requests_total",
			Help: "HTTP requests handled, by method, route pattern and status code.",
		}, []string{"method", "route", "status"}),
		httpDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace, Subsystem: "http", Name: "request_duration_seconds",
			Help:    "HTTP request latency, by method and route pattern.",
			Buckets: []float64{.001, .0025, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
		}, []string{"method", "route"}),
		httpInFlight: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace, Subsystem: "http", Name: "requests_in_flight",
			Help: "HTTP requests currently being served.",
		}),
		ordersPlaced: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace, Subsystem: "orders", Name: "placed_total",
			Help: "Orders placed successfully, by whether a coupon was applied.",
		}, []string{"coupon"}),
		orderRejections: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace, Subsystem: "orders", Name: "rejected_total",
			Help: "Orders rejected by validation, by reason.",
		}, []string{"reason"}),
		orderAmount: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace, Subsystem: "orders", Name: "total_amount",
			Help:    "Total of each placed order, after discount, in dollars.",
			Buckets: []float64{5, 10, 25, 50, 100, 250, 500, 1000, 5000, 10000},
		}),
		couponChecks: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace, Subsystem: "coupon", Name: "checks_total",
			Help: "Coupon codes checked against the index, by result.",
		}, []string{"result"}),
		authAttempts: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace, Subsystem: "auth", Name: "attempts_total",
			Help: "Authentication attempts, by method (password, bearer, api_key) and result.",
		}, []string{"method", "result"}),
		couponIndexCodes: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace, Subsystem: "coupon", Name: "index_codes",
			Help: "Valid codes in the loaded coupon index (0 when it is unavailable).",
		}),
		storage: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace, Name: "storage_info",
			Help: "Storage backing the API: 1 for the active mode.",
		}, []string{"mode"}),
	}
	m.registry.MustRegister(
		m.httpRequests, m.httpDuration, m.httpInFlight,
		m.ordersPlaced, m.orderRejections, m.orderAmount, m.couponChecks,
		m.authAttempts,
		m.couponIndexCodes, m.storage,
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	return m
}

// Handler serves the metrics in the Prometheus text format, for /metrics.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{Registry: m.registry})
}

// Registry returns the registry, for tests and extra collectors.
func (m *Metrics) Registry() *prometheus.Registry { return m.registry }

// RequestStarted counts a request as in flight.
func (m *Metrics) RequestStarted() { m.httpInFlight.Inc() }

// RequestFinished records a completed request. route is the matched ServeMux
// pattern, never the raw path, so the label set stays small.
func (m *Metrics) RequestFinished(method, route string, status int, took time.Duration) {
	m.httpInFlight.Dec()
	m.httpRequests.WithLabelValues(method, route, strconv.Itoa(status)).Inc()
	m.httpDuration.WithLabelValues(method, route).Observe(took.Seconds())
}

// OrderPlaced records a successful order and its total.
func (m *Metrics) OrderPlaced(withCoupon bool, total float64) {
	m.ordersPlaced.WithLabelValues(strconv.FormatBool(withCoupon)).Inc()
	m.orderAmount.Observe(total)
}

// OrderRejected records an order that failed validation.
func (m *Metrics) OrderRejected(reason string) {
	m.orderRejections.WithLabelValues(reason).Inc()
}

// CouponChecked records one coupon lookup.
func (m *Metrics) CouponChecked(valid bool) {
	result := "invalid"
	if valid {
		result = "valid"
	}
	m.couponChecks.WithLabelValues(result).Inc()
}

// AuthAttempt records one authentication attempt. method is "password"
// (POST /auth/token), "bearer" or "api_key".
func (m *Metrics) AuthAttempt(method string, ok bool) {
	result := "failure"
	if ok {
		result = "success"
	}
	m.authAttempts.WithLabelValues(method, result).Inc()
}

// SetCouponIndexCodes records how many valid codes the loaded index holds.
func (m *Metrics) SetCouponIndexCodes(n int) { m.couponIndexCodes.Set(float64(n)) }

// SetStorage records the active storage mode, e.g. "postgres".
func (m *Metrics) SetStorage(mode string) {
	m.storage.Reset()
	m.storage.WithLabelValues(mode).Set(1)
}
