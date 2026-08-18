// Package httpapi exposes the service over HTTP.
package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/convin/webhook-ingest/internal/ingest"
)

// Handler serves the webhook and stats endpoints.
type Handler struct {
	svc *ingest.Service
	log *slog.Logger
}

func (h *Handler) postCallWebhook(w http.ResponseWriter, r *http.Request) {
	var evt ingest.Event
	if err := json.NewDecoder(r.Body).Decode(&evt); err != nil {
		writeError(w, http.StatusBadRequest, "malformed json")
		return
	}
	if err := evt.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.svc.Ingest(r.Context(), evt); err != nil {
		h.log.Error("ingest failed", "event_id", evt.EventID, "err", err)
		writeError(w, http.StatusInternalServerError, "ingest failed")
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"accepted"}`))
}

func (h *Handler) getAccountStats(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("account_id")
	st := h.svc.Stats(accountID)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"account_id":         accountID,
		"call_count":         st.CallCount,
		"total_duration_sec": st.TotalDurationSec,
	})
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
