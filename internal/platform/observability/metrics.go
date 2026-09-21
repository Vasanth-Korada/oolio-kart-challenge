// Package observability defines the metrics seam used by the HTTP
// middleware and provides a Prometheus-backed implementation plus a
// no-op one for tests. Handlers and middleware depend only on
// MetricsRecorder, so the backend can change without touching them.
package observability

import "time"

// MetricsRecorder records one observation per completed HTTP request.
type MetricsRecorder interface {
	ObserveRequest(method, route string, status int, duration time.Duration)
}

// NoOp is a MetricsRecorder that discards everything — used in tests and
// anywhere metrics wiring would otherwise add noise.
type NoOp struct{}

func (NoOp) ObserveRequest(string, string, int, time.Duration) {}

var _ MetricsRecorder = NoOp{}
