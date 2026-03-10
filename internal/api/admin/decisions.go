package admin

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/aarnaud/crowdsec-central-api/internal/db/queries"
	"github.com/aarnaud/crowdsec-central-api/internal/models"
)

func ListDecisionsHandler(pool dbPool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		includeDeleted := r.URL.Query().Get("include_deleted") == "true"
		decisions, err := queries.ListDecisions(r.Context(), pool, includeDeleted)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if decisions == nil {
			decisions = []models.Decision{}
		}
		writeJSON(w, http.StatusOK, decisions)
	}
}

type CreateDecisionRequest struct {
	Type     string `json:"type"`
	Scope    string `json:"scope"`
	Value    string `json:"value"`
	Duration string `json:"duration"`
	Scenario string `json:"scenario"`
}

func CreateDecisionHandler(pool dbPool, defaultDuration time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CreateDecisionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		duration := defaultDuration
		if req.Duration != "" {
			if d, err := time.ParseDuration(req.Duration); err == nil {
				duration = d
			}
		}

		scenario := req.Scenario
		d := &models.Decision{
			Origin:    "manual",
			Type:      req.Type,
			Scope:     req.Scope,
			Value:     req.Value,
			Duration:  duration,
			Scenario:  &scenario,
			ExpiresAt: time.Now().Add(duration),
		}
		if err := queries.CreateDecision(r.Context(), pool, d); err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"message": "decision created"})
	}
}

func DeleteDecisionHandler(pool dbPool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uuid := chi.URLParam(r, "uuid")
		if err := queries.DeleteDecision(r.Context(), pool, uuid); err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
