package observability

import "time"

type MetricsRecorder interface {
	ObserveRequest(method, route string, status int, duration time.Duration)
}

type NoOp struct{}

func (NoOp) ObserveRequest(string, string, int, time.Duration) {}

var _ MetricsRecorder = NoOp{}
