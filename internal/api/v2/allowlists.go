package v2

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/aarnaud/crowdsec-central-api/internal/db/queries"
	"github.com/aarnaud/crowdsec-central-api/internal/models"
)

func allowlistToWire(a models.Allowlist, entries []models.AllowlistEntry) models.AllowlistResponseWire {
	items := make([]models.AllowlistItemWire, 0, len(entries))
	for _, e := range entries {
		item := models.AllowlistItemWire{
			Scope:     e.Scope,
			Value:     e.Value,
			CreatedAt: e.CreatedAt.UTC().Format("2006-01-02T15:04:05.000Z"),
		}
		if e.Comment != nil {
			item.Description = *e.Comment
		}
		if e.ExpiresAt != nil {
			item.Expiration = e.ExpiresAt.UTC().Format("2006-01-02T15:04:05.000Z")
		}
		items = append(items, item)
	}
	w := models.AllowlistResponseWire{
		AllowlistID:    fmt.Sprintf("%d", a.ID),
		ConsoleManaged: a.Managed,
		Name:           a.Name,
		CreatedAt:      a.CreatedAt.UTC().Format("2006-01-02T15:04:05.000Z"),
		UpdatedAt:      a.UpdatedAt.UTC().Format("2006-01-02T15:04:05.000Z"),
		Items:          items,
	}
	if a.Description != nil {
		w.Description = *a.Description
	}
	return w
}

// AllowlistsGetHandler handles GET /v3/allowlists[?with_content=true]
func AllowlistsGetHandler(pool dbPool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		withContent := r.URL.Query().Get("with_content") == "true"

		lists, err := queries.GetAllowlists(r.Context(), pool)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		result := make([]models.AllowlistResponseWire, 0, len(lists))
		for _, a := range lists {
			var entries []models.AllowlistEntry
			if withContent {
				entries, err = queries.GetAllowlistEntries(r.Context(), pool, a.ID)
				if err != nil {
					writeError(w, http.StatusInternalServerError, "internal error")
					return
				}
			}
			result = append(result, allowlistToWire(a, entries))
		}
		writeJSON(w, http.StatusOK, result)
	}
}

// AllowlistsHeadHandler handles HEAD /v3/allowlists
func AllowlistsHeadHandler(pool dbPool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}
}

// AllowlistGetByNameHandler handles GET /v3/allowlists/{name}[?with_content=true]
func AllowlistGetByNameHandler(pool dbPool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := chi.URLParam(r, "name")
		withContent := r.URL.Query().Get("with_content") == "true"

		a, err := queries.GetAllowlistByName(r.Context(), pool, name)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeError(w, http.StatusNotFound, "allowlist not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		var entries []models.AllowlistEntry
		if withContent {
			entries, err = queries.GetAllowlistEntries(r.Context(), pool, a.ID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "internal error")
				return
			}
		}
		writeJSON(w, http.StatusOK, allowlistToWire(*a, entries))
	}
}

// AllowlistCheckHeadHandler handles HEAD /v3/allowlists/check/{value}
func AllowlistCheckHeadHandler(pool dbPool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		value := chi.URLParam(r, "value")
		found, err := queries.IsAllowlisted(r.Context(), pool, "Ip", value)
		if err != nil || !found {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

// AllowlistCheckGetHandler handles GET /v3/allowlists/check/{value}
func AllowlistCheckGetHandler(pool dbPool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		value := chi.URLParam(r, "value")
		listName, comment, err := queries.CheckAllowlistedValue(r.Context(), pool, value)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeJSON(w, http.StatusOK, map[string]interface{}{"allowlisted": false})
				return
			}
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		reason := listName
		if comment != "" {
			reason = listName + ": " + comment
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"allowlisted": true, "reason": reason})
	}
}

// AllowlistCheckBulkHandler handles POST /v3/allowlists/check
func AllowlistCheckBulkHandler(pool dbPool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req models.BulkCheckAllowlistRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		results := make([]models.BulkCheckAllowlistResult, 0, len(req.Targets))
		for _, target := range req.Targets {
			listName, comment, err := queries.CheckAllowlistedValue(r.Context(), pool, target)
			result := models.BulkCheckAllowlistResult{Target: target, Allowlists: []string{}}
			if err == nil {
				entry := listName
				if comment != "" {
					entry = listName + ": " + comment
				}
				result.Allowlists = []string{entry}
			}
			results = append(results, result)
		}
		writeJSON(w, http.StatusOK, models.BulkCheckAllowlistResponse{Results: results})
	}
}

// AllowlistsPostHandler handles POST /v3/allowlists/{name} (sync from agent — no-op)
func AllowlistsPostHandler(pool dbPool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}
}
