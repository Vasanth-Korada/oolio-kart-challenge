package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"github.com/stretchr/testify/suite"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/auth"
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

// ContractSuite sends real requests through the full router and validates
// each request and response against api/openapi.yaml.
type ContractSuite struct {
	suite.Suite
	app  http.Handler
	spec routers.Router
}

func TestContractSuite(t *testing.T) {
	suite.Run(t, new(ContractSuite))
}

func (s *ContractSuite) SetupSuite() {
	s.spec = s.loadSpecRouter()
}

func (s *ContractSuite) SetupTest() {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	productRepo := product.NewMemoryRepository(product.SeedProducts())
	productService := product.NewService(productRepo, logger)

	orderRepo := order.NewMemoryRepository()
	couponValidator := fakeCouponValidator{valid: map[string]bool{"HAPPYHRS": true}}
	orderService := order.NewService(productService, couponValidator, orderRepo, nil, logger)

	users, err := auth.NewMemoryUserStore()
	s.Require().NoError(err)
	s.Require().NoError(users.Add("demo", "demo1234", auth.ScopeCreateOrder))
	tokens := newTestJWT(s.T())

	s.app = httpapi.NewRouter(httpapi.RouterDeps{
		Product:       &httpapi.ProductHandler{Service: productService},
		Order:         &httpapi.OrderHandler{Service: orderService},
		Health:        &httpapi.HealthHandler{DB: alwaysHealthyPinger{}},
		Logger:        logger,
		Auth:          &httpapi.AuthHandler{Users: users, Tokens: tokens},
		TokenVerifier: tokens,
		APIKey:        testAPIKey,
	})
}

// The spec's server is an absolute URL; overridden to "/" so requests
// route against our root-mounted app in this test.
func (s *ContractSuite) loadSpecRouter() routers.Router {
	ctx := context.Background()
	loader := &openapi3.Loader{Context: ctx, IsExternalRefsAllowed: true}
	doc, err := loader.LoadFromFile(specPath)
	s.Require().NoError(err, "load spec")
	doc.Servers = openapi3.Servers{{URL: "/"}}
	s.Require().NoError(doc.Validate(ctx), "spec failed self-validation")
	r, err := gorillamux.NewRouter(doc)
	s.Require().NoError(err, "build spec router")
	return r
}

// request builds a request; a non-nil body is sent as JSON.
func request(method, path string, body []byte, headers map[string]string) *http.Request {
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req := httptest.NewRequest(method, path, r)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return req
}

// assertConformant validates req against the spec, serves it, validates the
// response, and checks the status. body is a separate copy because
// validating consumes the request body.
func (s *ContractSuite) assertConformant(req *http.Request, body []byte, wantStatus int) *httptest.ResponseRecorder {
	ctx := context.Background()

	validationReq := req.Clone(ctx)
	if body != nil {
		validationReq.Body = io.NopCloser(bytes.NewReader(body))
	}

	route, pathParams, err := s.spec.FindRoute(validationReq)
	s.Require().NoError(err, "spec has no route for %s %s", req.Method, req.URL.Path)

	reqInput := &openapi3filter.RequestValidationInput{
		Request:    validationReq,
		PathParams: pathParams,
		Route:      route,
		// Credential enforcement is checked via status codes below, not here.
		Options: &openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc},
	}
	s.Require().NoError(openapi3filter.ValidateRequest(ctx, reqInput), "request does not conform to spec")

	rec := httptest.NewRecorder()
	s.app.ServeHTTP(rec, req)

	respInput := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: reqInput,
		Status:                 rec.Code,
		Header:                 rec.Header(),
	}
	respInput.SetBodyBytes(rec.Body.Bytes())
	s.NoError(openapi3filter.ValidateResponse(ctx, respInput), "response does not conform to spec\nbody: %s", rec.Body.String())
	s.Equal(wantStatus, rec.Code, "body=%s", rec.Body.String())
	return rec
}

// login gets an access token through POST /auth/token.
func (s *ContractSuite) login() string {
	body := []byte(`{"username":"demo","password":"demo1234"}`)
	rec := s.assertConformant(request(http.MethodPost, "/auth/token", body, nil), body, http.StatusOK)
	var resp struct {
		AccessToken string `json:"accessToken"`
	}
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &resp))
	s.Require().NotEmpty(resp.AccessToken)
	return resp.AccessToken
}

func (s *ContractSuite) TestProducts() {
	tests := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{name: "list", path: "/product", wantStatus: http.StatusOK},
		{name: "get one", path: "/product/1", wantStatus: http.StatusOK},
		{name: "not found", path: "/product/999999", wantStatus: http.StatusNotFound},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.assertConformant(request(http.MethodGet, tt.path, nil, nil), nil, tt.wantStatus)
		})
	}
}

// productId is spec'd as an integer, so kin-openapi rejects this at
// request validation, before any response exists to check.
func (s *ContractSuite) TestGetProductInvalidID() {
	req := httptest.NewRequest(http.MethodGet, "/product/not-a-number", nil)

	route, pathParams, err := s.spec.FindRoute(req)
	s.Require().NoError(err)
	reqInput := &openapi3filter.RequestValidationInput{Request: req, PathParams: pathParams, Route: route}
	s.Error(openapi3filter.ValidateRequest(context.Background(), reqInput),
		"the spec's integer productId constraint should reject a non-numeric id")

	rec := httptest.NewRecorder()
	s.app.ServeHTTP(rec, req)
	s.Equal(http.StatusBadRequest, rec.Code)
}

func (s *ContractSuite) TestPlaceOrderWithAPIKey() {
	tests := []struct {
		name       string
		body       string
		headers    map[string]string
		wantStatus int
	}{
		{name: "success", body: `{"items":[{"productId":"1","quantity":2}],"couponCode":"HAPPYHRS"}`, headers: map[string]string{"api_key": testAPIKey}, wantStatus: http.StatusOK},
		{name: "missing credentials", body: `{"items":[{"productId":"1","quantity":1}]}`, wantStatus: http.StatusUnauthorized},
		{name: "wrong api key", body: `{"items":[{"productId":"1","quantity":1}]}`, headers: map[string]string{"api_key": "wrong-key"}, wantStatus: http.StatusForbidden},
		{name: "invalid coupon", body: `{"items":[{"productId":"1","quantity":1}],"couponCode":"NOTVALID1"}`, headers: map[string]string{"api_key": testAPIKey}, wantStatus: http.StatusUnprocessableEntity},
		{name: "empty items", body: `{"items":[]}`, headers: map[string]string{"api_key": testAPIKey}, wantStatus: http.StatusUnprocessableEntity},
		{name: "101 lines", body: `{"items":[` + strings.TrimSuffix(strings.Repeat(`{"productId":"1","quantity":1},`, 101), ",") + `]}`, headers: map[string]string{"api_key": testAPIKey}, wantStatus: http.StatusUnprocessableEntity},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			body := []byte(tt.body)
			s.assertConformant(request(http.MethodPost, "/order", body, tt.headers), body, tt.wantStatus)
		})
	}
}

func (s *ContractSuite) TestAuthToken() {
	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{name: "wrong password", body: `{"username":"demo","password":"nope"}`, wantStatus: http.StatusUnauthorized},
		{name: "unknown user", body: `{"username":"ghost","password":"demo1234"}`, wantStatus: http.StatusUnauthorized},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			body := []byte(tt.body)
			s.assertConformant(request(http.MethodPost, "/auth/token", body, nil), body, tt.wantStatus)
		})
	}
	s.Run("success", func() { s.login() })
}

func (s *ContractSuite) TestPlaceOrderWithBearerToken() {
	body := []byte(`{"items":[{"productId":"1","quantity":1}]}`)

	s.Run("token from /auth/token places an order", func() {
		headers := map[string]string{"Authorization": "Bearer " + s.login()}
		s.assertConformant(request(http.MethodPost, "/order", body, headers), body, http.StatusOK)
	})

	s.Run("tampered token is 401", func() {
		headers := map[string]string{"Authorization": "Bearer " + s.login() + "x"}
		rec := s.assertConformant(request(http.MethodPost, "/order", body, headers), body, http.StatusUnauthorized)
		s.Contains(rec.Header().Get("WWW-Authenticate"), `error="invalid_token"`)
	})
}
