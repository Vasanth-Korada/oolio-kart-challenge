package httpapi_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/httpapi"
)

type fakePinger struct{ err error }

func (f fakePinger) Ping(context.Context) error { return f.err }

func TestHealthHandler_Live(t *testing.T) {
	h := &httpapi.HealthHandler{DB: fakePinger{err: errors.New("irrelevant: liveness never pings")}}
	rec := httptest.NewRecorder()
	h.Live(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestHealthHandler_Ready(t *testing.T) {
	tests := []struct {
		name       string
		db         fakePinger
		storage    string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "postgres reachable",
			db:         fakePinger{},
			storage:    "postgres",
			wantStatus: http.StatusOK,
			wantBody:   `{"status":"ready","storage":"postgres"}`,
		},
		{
			name:       "in-memory fallback reports ready too, but says so",
			db:         fakePinger{},
			storage:    "in-memory (fallback)",
			wantStatus: http.StatusOK,
			wantBody:   `{"status":"ready","storage":"in-memory (fallback)"}`,
		},
		{
			name:       "postgres unreachable",
			db:         fakePinger{err: errors.New("connection refused")},
			storage:    "postgres",
			wantStatus: http.StatusServiceUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &httpapi.HealthHandler{DB: tt.db, Storage: tt.storage}
			rec := httptest.NewRecorder()
			h.Ready(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d, body=%s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.wantBody != "" {
				if got := rec.Body.String(); got != tt.wantBody+"\n" {
					t.Fatalf("body = %s, want %s", got, tt.wantBody)
				}
			}
		})
	}
}
