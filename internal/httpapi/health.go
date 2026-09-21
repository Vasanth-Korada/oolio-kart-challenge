package httpapi

import (
	"context"
	"net/http"
)

// Pinger is satisfied by *pgxpool.Pool. Defined here (rather than
// depending on pgx directly) so health handlers stay testable with a
// fake and agnostic to the storage backend.
type Pinger interface {
	Ping(ctx context.Context) error
}

// HealthHandler answers liveness/readiness probes.
type HealthHandler struct {
	DB Pinger
}

// Live reports whether the process is up at all — always 200 once the
// server can serve requests. Kubernetes-style liveness probes hit this.
func (h *HealthHandler) Live(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Ready reports whether the process can actually serve traffic — it
// pings the database so a pod isn't sent traffic before its DB
// connection is up, or is pulled out once the DB becomes unreachable.
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	if err := h.DB.Ping(r.Context()); err != nil {
		WriteError(w, http.StatusServiceUnavailable, "not_ready", "database is unreachable")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
