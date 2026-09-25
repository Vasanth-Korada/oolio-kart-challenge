package httpapi

import (
	"net/http"
	"time"
)

// HTTPMetrics receives one event when a request starts and one when it
// finishes. Implementations must be safe for concurrent use.
type HTTPMetrics interface {
	RequestStarted()
	RequestFinished(method, route string, status int, took time.Duration)
}

// AuthMetrics counts authentication attempts by method ("password",
// "bearer" or "api_key") and outcome. Implementations must be safe for
// concurrent use.
type AuthMetrics interface {
	AuthAttempt(method string, ok bool)
}

// noAuthMetrics is used when RouterDeps.AuthMetrics is nil.
type noAuthMetrics struct{}

func (noAuthMetrics) AuthAttempt(string, bool) {}

// unmatchedRoute labels requests no route matched (404s, wrong methods,
// CORS preflights), so raw paths never become label values.
const unmatchedRoute = "unmatched"

// Metrics records every request's method, route pattern, status and
// latency. routeOf maps a request to its ServeMux pattern ("" if none); it
// is resolved before the request is served, because inner middleware passes
// copies of the request down and the mux's pattern never reaches this layer.
// Requests for skipRoute (the metrics endpoint itself) are not recorded.
func Metrics(recorder HTTPMetrics, routeOf func(*http.Request) string, skipRoute string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			route := routeOf(r)
			if route == skipRoute {
				next.ServeHTTP(w, r)
				return
			}
			if route == "" {
				route = unmatchedRoute
			}
			start := time.Now()
			recorder.RequestStarted()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			defer func() {
				recorder.RequestFinished(r.Method, route, rec.status, time.Since(start))
			}()
			next.ServeHTTP(rec, r)
		})
	}
}
