package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/convin/webhook-ingest/internal/ingest"
)

// NewRouter wires every route the service serves.
func NewRouter(svc *ingest.Service, log *slog.Logger) http.Handler {
	h := &Handler{svc: svc, log: log}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("POST /webhooks/calls", h.postCallWebhook)
	mux.HandleFunc("GET /accounts/{account_id}/stats", h.getAccountStats)
	return mux
}
