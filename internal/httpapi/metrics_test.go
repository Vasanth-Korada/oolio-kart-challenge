package httpapi_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/httpapi"
	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/order"
	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/product"
)

type recordedRequest struct {
	method, route string
	status        int
}

// fakeHTTPMetrics records what the Metrics middleware reports.
type fakeHTTPMetrics struct {
	mu       sync.Mutex
	inFlight int
	finished []recordedRequest
}

func (f *fakeHTTPMetrics) RequestStarted() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.inFlight++
}

func (f *fakeHTTPMetrics) RequestFinished(method, route string, status int, _ time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.inFlight--
	f.finished = append(f.finished, recordedRequest{method, route, status})
}

type MetricsMiddlewareSuite struct {
	suite.Suite
	metrics *fakeHTTPMetrics
	router  http.Handler
}

func TestMetricsMiddlewareSuite(t *testing.T) {
	suite.Run(t, new(MetricsMiddlewareSuite))
}

func (s *MetricsMiddlewareSuite) SetupTest() {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	products := product.NewService(product.NewMemoryRepository(product.SeedProducts()), logger)
	orders := order.NewService(products, fakeCouponValidator{}, defaultDiscounts(), order.NewMemoryRepository(), nil, logger)
	s.metrics = &fakeHTTPMetrics{}
	s.router = httpapi.NewRouter(httpapi.RouterDeps{
		Product: &httpapi.ProductHandler{Service: products},
		Order:   &httpapi.OrderHandler{Service: orders},
		Health:  &httpapi.HealthHandler{DB: alwaysHealthyPinger{}},
		Logger:  logger,
		APIKey:  testAPIKey,

		Metrics: s.metrics,
		MetricsHandler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, "metrics")
		}),
	})
}

func (s *MetricsMiddlewareSuite) serve(method, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	s.router.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader("")))
	return rec
}

func (s *MetricsMiddlewareSuite) TestLabelsRequestsByRoutePatternAndStatus() {
	tests := []struct {
		name   string
		method string
		path   string
		want   recordedRequest
	}{
		{name: "path parameter becomes the pattern", method: http.MethodGet, path: "/product/5",
			want: recordedRequest{"GET", "GET /product/{id}", http.StatusOK}},
		{name: "not found keeps the route label", method: http.MethodGet, path: "/product/999",
			want: recordedRequest{"GET", "GET /product/{id}", http.StatusNotFound}},
		{name: "auth failure is labelled by its route", method: http.MethodPost, path: "/order",
			want: recordedRequest{"POST", "POST /order", http.StatusUnauthorized}},
		{name: "unknown path never becomes a label", method: http.MethodGet, path: "/no/such/path",
			want: recordedRequest{"GET", "unmatched", http.StatusNotFound}},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.SetupTest()

			s.serve(tt.method, tt.path)

			s.Equal([]recordedRequest{tt.want}, s.metrics.finished)
			s.Zero(s.metrics.inFlight, "every started request must finish")
		})
	}
}

func (s *MetricsMiddlewareSuite) TestMetricsEndpointIsServedButNotRecorded() {
	rec := s.serve(http.MethodGet, "/metrics")

	s.Equal(http.StatusOK, rec.Code)
	s.Equal("metrics", rec.Body.String())
	s.Empty(s.metrics.finished)
}

func (s *MetricsMiddlewareSuite) TestRecoveredPanicCountsAsServerError() {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	panicking := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") })
	h := httpapi.Metrics(s.metrics, func(*http.Request) string { return "GET /boom" }, "GET /metrics")(
		httpapi.Recover(logger)(panicking))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/boom", nil))

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal([]recordedRequest{{"GET", "GET /boom", http.StatusInternalServerError}}, s.metrics.finished)
}

func (s *MetricsMiddlewareSuite) TestRouterWithoutMetricsHasNoEndpoint() {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	products := product.NewService(product.NewMemoryRepository(product.SeedProducts()), logger)
	router := httpapi.NewRouter(httpapi.RouterDeps{
		Product: &httpapi.ProductHandler{Service: products},
		Order:   &httpapi.OrderHandler{},
		Health:  &httpapi.HealthHandler{DB: alwaysHealthyPinger{}},
		Logger:  logger,
	})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	s.Equal(http.StatusNotFound, rec.Code)
}
