package metrics_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/suite"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/httpapi"
	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/order"
	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/platform/metrics"
)

// Compile-time checks: Metrics satisfies the interfaces its consumers own.
var (
	_ httpapi.HTTPMetrics = (*metrics.Metrics)(nil)
	_ order.Recorder      = (*metrics.Metrics)(nil)
)

type MetricsSuite struct {
	suite.Suite
	m *metrics.Metrics
}

func TestMetricsSuite(t *testing.T) {
	suite.Run(t, new(MetricsSuite))
}

func (s *MetricsSuite) SetupTest() {
	s.m = metrics.New()
}

// scrape returns the /metrics body as Prometheus would receive it.
func (s *MetricsSuite) scrape() string {
	rec := httptest.NewRecorder()
	s.m.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	s.Require().Equal(http.StatusOK, rec.Code)
	body, err := io.ReadAll(rec.Body)
	s.Require().NoError(err)
	return string(body)
}

func (s *MetricsSuite) TestHTTPRequests() {
	s.m.RequestStarted()
	s.m.RequestStarted()
	s.m.RequestFinished("GET", "GET /product/{id}", 200, 3*time.Millisecond)

	body := s.scrape()

	s.Contains(body, `oolio_http_requests_total{method="GET",route="GET /product/{id}",status="200"} 1`)
	s.Contains(body, `oolio_http_request_duration_seconds_count{method="GET",route="GET /product/{id}"} 1`)
	s.Contains(body, `oolio_http_request_duration_seconds_bucket{method="GET",route="GET /product/{id}",le="0.005"} 1`)
	s.Contains(body, "oolio_http_requests_in_flight 1")
}

func (s *MetricsSuite) TestOrderEvents() {
	s.m.OrderPlaced(true, 26.12)
	s.m.OrderPlaced(false, 10)
	s.m.OrderRejected("invalid_coupon")
	s.m.OrderRejected("invalid_coupon")
	s.m.CouponChecked(true)
	s.m.CouponChecked(false)

	body := s.scrape()

	s.Contains(body, `oolio_orders_placed_total{coupon="true"} 1`)
	s.Contains(body, `oolio_orders_placed_total{coupon="false"} 1`)
	s.Contains(body, `oolio_orders_rejected_total{reason="invalid_coupon"} 2`)
	s.Contains(body, "oolio_orders_total_amount_sum 36.12")
	s.Contains(body, "oolio_orders_total_amount_count 2")
	s.Contains(body, `oolio_coupon_checks_total{result="valid"} 1`)
	s.Contains(body, `oolio_coupon_checks_total{result="invalid"} 1`)
}

func (s *MetricsSuite) TestStartupGauges() {
	s.m.SetCouponIndexCodes(8)
	s.m.SetStorage("in-memory (fallback)")
	s.m.SetStorage("postgres")

	body := s.scrape()

	s.Contains(body, "oolio_coupon_index_codes 8")
	s.Contains(body, `oolio_storage_info{mode="postgres"} 1`)
	s.NotContains(body, "in-memory", "only the active storage mode is reported")
}

func (s *MetricsSuite) TestIncludesGoRuntimeAndProcessMetrics() {
	body := s.scrape()

	s.Contains(body, "go_goroutines")
	s.Contains(body, "go_memstats_heap_inuse_bytes")
	s.Contains(body, "process_cpu_seconds_total")
}

func (s *MetricsSuite) TestInstancesAreIndependent() {
	other := metrics.New()
	s.m.OrderRejected("empty_items")

	s.Equal(0, testutil.CollectAndCount(other.Registry(), "oolio_orders_rejected_total"))
	s.Equal(1, testutil.CollectAndCount(s.m.Registry(), "oolio_orders_rejected_total"))
}
