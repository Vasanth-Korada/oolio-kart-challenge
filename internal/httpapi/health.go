package httpapi

import (
	"context"
	"net/http"
)

type Pinger interface {
	Ping(ctx context.Context) error
}

type HealthHandler struct {
	DB Pinger
}

func (h *HealthHandler) Live(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	if err := h.DB.Ping(r.Context()); err != nil {
		WriteError(w, http.StatusServiceUnavailable, "not_ready", "database is unreachable")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
