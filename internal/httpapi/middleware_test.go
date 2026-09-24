package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/auth"
	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/httpapi"
)

// testJWTSecret is shared by the router and the tests that mint tokens.
var testJWTSecret = []byte("test-secret-test-secret-test-sec")

func newTestJWT(t *testing.T) *auth.JWTManager {
	t.Helper()
	m, err := auth.NewJWTManager(testJWTSecret, time.Minute)
	if err != nil {
		t.Fatalf("new jwt manager: %v", err)
	}
	return m
}

// fakeAuthMetrics records AuthAttempt calls as "method:ok".
type fakeAuthMetrics struct{ calls []string }

func (f *fakeAuthMetrics) AuthAttempt(method string, ok bool) {
	result := "failure"
	if ok {
		result = "success"
	}
	f.calls = append(f.calls, method+":"+result)
}

type MiddlewareSuite struct {
	suite.Suite
	jwt *auth.JWTManager
}

func TestMiddlewareSuite(t *testing.T) {
	suite.Run(t, new(MiddlewareSuite))
}

func (s *MiddlewareSuite) SetupTest() {
	s.jwt = newTestJWT(s.T())
}

func (s *MiddlewareSuite) token(scopes ...string) string {
	tok, err := s.jwt.Issue("demo", scopes)
	s.Require().NoError(err)
	return tok.Value
}

// orderGuard is the POST /order middleware stack around a handler that
// echoes the authenticated subject.
func (s *MiddlewareSuite) orderGuard(m httpapi.AuthMetrics) http.Handler {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, _ := httpapi.ClaimsFromContext(r.Context())
		w.Header().Set("X-Subject", claims.Subject)
		w.WriteHeader(http.StatusOK)
	})
	return httpapi.Authenticate(s.jwt, testAPIKey, m)(httpapi.RequireScope(auth.ScopeCreateOrder)(ok))
}

func (s *MiddlewareSuite) TestAuthenticateAndRequireScope() {
	otherIssuer, err := auth.NewJWTManager([]byte("another-secret-another-secret-xx"), time.Minute)
	s.Require().NoError(err)
	forged, err := otherIssuer.Issue("demo", []string{auth.ScopeCreateOrder})
	s.Require().NoError(err)

	tests := []struct {
		name          string
		headers       map[string]string
		wantStatus    int
		wantSubject   string
		wantChallenge bool
		wantMetric    string
	}{
		{name: "no credentials", wantStatus: http.StatusUnauthorized, wantChallenge: true},
		{name: "api key: correct", headers: map[string]string{"api_key": testAPIKey}, wantStatus: http.StatusOK, wantSubject: "api_key", wantMetric: "api_key:success"},
		{name: "api key: wrong", headers: map[string]string{"api_key": "wrong"}, wantStatus: http.StatusForbidden, wantMetric: "api_key:failure"},
		{name: "bearer: valid token with scope", headers: map[string]string{"Authorization": "Bearer " + s.token(auth.ScopeCreateOrder)}, wantStatus: http.StatusOK, wantSubject: "demo", wantMetric: "bearer:success"},
		{name: "bearer: scheme is case-insensitive", headers: map[string]string{"Authorization": "bearer " + s.token(auth.ScopeCreateOrder)}, wantStatus: http.StatusOK, wantSubject: "demo", wantMetric: "bearer:success"},
		{name: "bearer: valid token without scope", headers: map[string]string{"Authorization": "Bearer " + s.token("read")}, wantStatus: http.StatusForbidden, wantMetric: "bearer:success"},
		{name: "bearer: signed with another secret", headers: map[string]string{"Authorization": "Bearer " + forged.Value}, wantStatus: http.StatusUnauthorized, wantChallenge: true, wantMetric: "bearer:failure"},
		{name: "bearer: garbage token", headers: map[string]string{"Authorization": "Bearer abc.def.ghi"}, wantStatus: http.StatusUnauthorized, wantChallenge: true, wantMetric: "bearer:failure"},
		{name: "bearer: empty token", headers: map[string]string{"Authorization": "Bearer "}, wantStatus: http.StatusUnauthorized, wantChallenge: true, wantMetric: "bearer:failure"},
		{name: "basic scheme is not accepted", headers: map[string]string{"Authorization": "Basic ZGVtbzpkZW1vMTIzNA=="}, wantStatus: http.StatusUnauthorized, wantChallenge: true, wantMetric: "bearer:failure"},
		{
			name:       "bearer wins over api key: bad token, good key",
			headers:    map[string]string{"Authorization": "Bearer nope", "api_key": testAPIKey},
			wantStatus: http.StatusUnauthorized, wantChallenge: true, wantMetric: "bearer:failure",
		},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			m := &fakeAuthMetrics{}
			req := httptest.NewRequest(http.MethodPost, "/order", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()

			s.orderGuard(m).ServeHTTP(rec, req)

			s.Equal(tt.wantStatus, rec.Code, "body=%s", rec.Body.String())
			s.Equal(tt.wantSubject, rec.Header().Get("X-Subject"))
			if tt.wantChallenge {
				s.Contains(rec.Header().Get("WWW-Authenticate"), "Bearer")
			}
			if tt.wantStatus != http.StatusOK {
				s.Contains(rec.Header().Get("Content-Type"), "application/json", "errors use the JSON envelope")
			}
			if tt.wantMetric == "" {
				s.Empty(m.calls)
			} else {
				s.Equal([]string{tt.wantMetric}, m.calls)
			}
		})
	}
}

func (s *MiddlewareSuite) TestRequireScopeWithoutAuthenticateIs401() {
	h := httpapi.RequireScope(auth.ScopeCreateOrder)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.Fail("handler must not run")
	}))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/order", nil))

	s.Equal(http.StatusUnauthorized, rec.Code)
}

func (s *MiddlewareSuite) TestAuthenticateAcceptsNilMetrics() {
	req := httptest.NewRequest(http.MethodPost, "/order", nil)
	req.Header.Set("api_key", testAPIKey)
	rec := httptest.NewRecorder()

	s.orderGuard(nil).ServeHTTP(rec, req)

	s.Equal(http.StatusOK, rec.Code)
}

func (s *MiddlewareSuite) TestCORS() {
	handler := httpapi.CORS("*")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	s.Run("preflight gets a 204 with the CORS headers", func() {
		req := httptest.NewRequest(http.MethodOptions, "/order", nil)
		req.Header.Set("Origin", "http://localhost:5173")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		s.Equal(http.StatusNoContent, rec.Code)
		s.Equal("*", rec.Header().Get("Access-Control-Allow-Origin"))
		s.Contains(rec.Header().Get("Access-Control-Allow-Headers"), "Authorization")
		s.Contains(rec.Header().Get("Access-Control-Allow-Headers"), "api_key")
	})

	s.Run("actual request also carries the headers", func() {
		req := httptest.NewRequest(http.MethodGet, "/product", nil)
		req.Header.Set("Origin", "http://localhost:5173")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		s.Equal(http.StatusOK, rec.Code)
		s.Equal("*", rec.Header().Get("Access-Control-Allow-Origin"))
	})

	s.Run("no Origin header, no CORS headers set", func() {
		req := httptest.NewRequest(http.MethodGet, "/product", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		s.Empty(rec.Header().Get("Access-Control-Allow-Origin"))
	})
}

func (s *MiddlewareSuite) TestRequestID() {
	var gotID string
	handler := httpapi.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotID = httpapi.RequestIDFromContext(r.Context())
	}))

	s.Run("generates an id when none supplied", func() {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/product", nil))

		s.NotEmpty(gotID)
		s.Equal(gotID, rec.Header().Get("X-Request-Id"))
	})

	s.Run("preserves a caller-supplied id", func() {
		req := httptest.NewRequest(http.MethodGet, "/product", nil)
		req.Header.Set("X-Request-Id", "caller-id-123")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		s.Equal("caller-id-123", gotID)
	})
}
