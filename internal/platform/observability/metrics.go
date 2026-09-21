// Package observability defines the metrics seam used by the HTTP
// middleware, with a Prometheus-backed implementation and a no-op one.
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
