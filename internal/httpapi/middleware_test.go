package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/httpapi"
)

func TestAPIKeyAuth(t *testing.T) {
	handler := httpapi.APIKeyAuth(testAPIKey)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name       string
		headerKey  string
		wantStatus int
	}{
		{name: "missing key", headerKey: "", wantStatus: http.StatusUnauthorized},
		{name: "wrong key", headerKey: "wrong", wantStatus: http.StatusForbidden},
		{name: "correct key", headerKey: testAPIKey, wantStatus: http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/order", nil)
			if tt.headerKey != "" {
				req.Header.Set("api_key", tt.headerKey)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

func TestCORS(t *testing.T) {
	handler := httpapi.CORS("*")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("preflight gets a 204 with the CORS headers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/order", nil)
		req.Header.Set("Origin", "http://localhost:5173")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", rec.Code)
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
			t.Fatalf("Access-Control-Allow-Origin = %q, want *", got)
		}
	})

	t.Run("actual request also carries the headers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/product", nil)
		req.Header.Set("Origin", "http://localhost:5173")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
			t.Fatalf("Access-Control-Allow-Origin = %q, want *", got)
		}
	})

	t.Run("no Origin header, no CORS headers set", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/product", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Fatalf("Access-Control-Allow-Origin = %q, want empty", got)
		}
	})
}

func TestRequestID(t *testing.T) {
	var gotID string
	handler := httpapi.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotID = httpapi.RequestIDFromContext(r.Context())
	}))

	t.Run("generates an id when none supplied", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/product", nil))

		if gotID == "" {
			t.Fatal("expected a generated request id")
		}
		if rec.Header().Get("X-Request-Id") != gotID {
			t.Fatalf("X-Request-Id header = %q, want %q", rec.Header().Get("X-Request-Id"), gotID)
		}
	})

	t.Run("preserves a caller-supplied id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/product", nil)
		req.Header.Set("X-Request-Id", "caller-id-123")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if gotID != "caller-id-123" {
			t.Fatalf("request id = %q, want caller-id-123", gotID)
		}
	})
}
