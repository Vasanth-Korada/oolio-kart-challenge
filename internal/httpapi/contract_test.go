package httpapi_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/httpapi"
	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/order"
	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/product"
)

const (
	specPath   = "../../api/openapi.yaml"
	testAPIKey = "apitest"
)

type fakeCouponValidator struct{ valid map[string]bool }

func (f fakeCouponValidator) IsValid(code string) bool { return f.valid[code] }

type alwaysHealthyPinger struct{}

func (alwaysHealthyPinger) Ping(context.Context) error { return nil }

func newTestRouter(t *testing.T) http.Handler {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	productRepo := product.NewMemoryRepository(product.SeedProducts())
	productService := product.NewService(productRepo, logger)

	orderRepo := order.NewMemoryRepository()
	couponValidator := fakeCouponValidator{valid: map[string]bool{"HAPPYHRS": true}}
	orderService := order.NewService(productService, couponValidator, orderRepo, logger)

	return httpapi.NewRouter(httpapi.RouterDeps{
		Product: &httpapi.ProductHandler{Service: productService},
		Order:   &httpapi.OrderHandler{Service: orderService},
		Health:  &httpapi.HealthHandler{DB: alwaysHealthyPinger{}},
		Logger:  logger,
		APIKey:  testAPIKey,
	})
}

// The spec's server is an absolute URL; overridden to "/" so requests
// route against our root-mounted app in this test.
func loadSpecRouter(t *testing.T) routers.Router {
	t.Helper()
	ctx := context.Background()
	loader := &openapi3.Loader{Context: ctx, IsExternalRefsAllowed: true}
	doc, err := loader.LoadFromFile(specPath)
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}
	doc.Servers = openapi3.Servers{{URL: "/"}}
	if err := doc.Validate(ctx); err != nil {
		t.Fatalf("spec failed self-validation: %v", err)
	}
	r, err := gorillamux.NewRouter(doc)
	if err != nil {
		t.Fatalf("build spec router: %v", err)
	}
	return r
}

// bodyForValidation is a separate copy: validating consumes the body.
func assertConformant(t *testing.T, app http.Handler, spec routers.Router, req *http.Request, bodyForValidation []byte) *httptest.ResponseRecorder {
	t.Helper()
	ctx := context.Background()

	validationReq := req.Clone(ctx)
	if bodyForValidation != nil {
		validationReq.Body = io.NopCloser(bytes.NewReader(bodyForValidation))
	}

	route, pathParams, err := spec.FindRoute(validationReq)
	if err != nil {
		t.Fatalf("spec has no route for %s %s: %v", req.Method, req.URL.Path, err)
	}

	reqInput := &openapi3filter.RequestValidationInput{
		Request:    validationReq,
		PathParams: pathParams,
		Route:      route,
		// api_key enforcement is checked via status codes below, not here.
		Options: &openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc},
	}
	if err := openapi3filter.ValidateRequest(ctx, reqInput); err != nil {
		t.Fatalf("request does not conform to spec: %v", err)
	}

	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	respInput := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: reqInput,
		Status:                 rec.Code,
		Header:                 rec.Header(),
	}
	respInput.SetBodyBytes(rec.Body.Bytes())
	if err := openapi3filter.ValidateResponse(ctx, respInput); err != nil {
		t.Errorf("response does not conform to spec: %v\nbody: %s", err, rec.Body.String())
	}
	return rec
}

func TestContract_ListProducts(t *testing.T) {
	app, spec := newTestRouter(t), loadSpecRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/product", nil)
	if rec := assertConformant(t, app, spec, req, nil); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestContract_GetProduct(t *testing.T) {
	app, spec := newTestRouter(t), loadSpecRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/product/1", nil)
	if rec := assertConformant(t, app, spec, req, nil); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

// productId is spec'd as an integer, so kin-openapi rejects this at
// request validation, before any response exists to check.
func TestContract_GetProduct_InvalidID(t *testing.T) {
	spec := loadSpecRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/product/not-a-number", nil)

	route, pathParams, err := spec.FindRoute(req)
	if err != nil {
		t.Fatalf("spec has no route for %s: %v", req.URL.Path, err)
	}
	reqInput := &openapi3filter.RequestValidationInput{Request: req, PathParams: pathParams, Route: route}
	if err := openapi3filter.ValidateRequest(context.Background(), reqInput); err == nil {
		t.Fatal("expected the spec's integer productId constraint to reject a non-numeric id, got no error")
	}

	app := newTestRouter(t)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("app status = %d, want 400", rec.Code)
	}
}

func TestContract_GetProduct_NotFound(t *testing.T) {
	app, spec := newTestRouter(t), loadSpecRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/product/999999", nil)
	if rec := assertConformant(t, app, spec, req, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestContract_PlaceOrder_Success(t *testing.T) {
	app, spec := newTestRouter(t), loadSpecRouter(t)

	body := []byte(`{"items":[{"productId":"1","quantity":2}],"couponCode":"HAPPYHRS"}`)
	req := httptest.NewRequest(http.MethodPost, "/order", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api_key", testAPIKey)

	if rec := assertConformant(t, app, spec, req, body); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
}

func TestContract_PlaceOrder_MissingAPIKey(t *testing.T) {
	app, spec := newTestRouter(t), loadSpecRouter(t)

	body := []byte(`{"items":[{"productId":"1","quantity":1}]}`)
	req := httptest.NewRequest(http.MethodPost, "/order", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	if rec := assertConformant(t, app, spec, req, body); rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestContract_PlaceOrder_WrongAPIKey(t *testing.T) {
	app, spec := newTestRouter(t), loadSpecRouter(t)

	body := []byte(`{"items":[{"productId":"1","quantity":1}]}`)
	req := httptest.NewRequest(http.MethodPost, "/order", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api_key", "wrong-key")

	if rec := assertConformant(t, app, spec, req, body); rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestContract_PlaceOrder_InvalidCoupon(t *testing.T) {
	app, spec := newTestRouter(t), loadSpecRouter(t)

	body := []byte(`{"items":[{"productId":"1","quantity":1}],"couponCode":"NOTVALID1"}`)
	req := httptest.NewRequest(http.MethodPost, "/order", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api_key", testAPIKey)

	if rec := assertConformant(t, app, spec, req, body); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
}

func TestContract_PlaceOrder_EmptyItems(t *testing.T) {
	app, spec := newTestRouter(t), loadSpecRouter(t)

	body := []byte(`{"items":[]}`)
	req := httptest.NewRequest(http.MethodPost, "/order", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api_key", testAPIKey)

	if rec := assertConformant(t, app, spec, req, body); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
}
