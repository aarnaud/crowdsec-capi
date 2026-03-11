package admin

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/aarnaud/crowdsec-central-api/internal/db/queries"
	"github.com/aarnaud/crowdsec-central-api/internal/models"
	"github.com/aarnaud/crowdsec-central-api/internal/validation"
)

func ListAllowlistsHandler(pool dbPool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		lists, err := queries.GetAllowlists(r.Context(), pool)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		writeJSON(w, http.StatusOK, lists)
	}
}

type CreateAllowlistRequest struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

func CreateAllowlistHandler(pool dbPool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CreateAllowlistRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Name == "" || len(req.Name) > 128 {
			writeError(w, http.StatusBadRequest, "name is required and must be ≤ 128 characters")
			return
		}
		list, err := queries.CreateAllowlist(r.Context(), pool, req.Name, req.Label, req.Description)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		writeJSON(w, http.StatusCreated, list)
	}
}

func DeleteAllowlistHandler(pool dbPool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid id")
			return
		}
		if err := queries.DeleteAllowlist(r.Context(), pool, id); err != nil {
			if err.Error() == "cannot delete a managed allowlist (remove it from the allowlists file)" {
				writeError(w, http.StatusConflict, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

type AddAllowlistEntryRequest struct {
	Scope   string `json:"scope"`
	Value   string `json:"value"`
	Comment string `json:"comment"`
}

func ListAllowlistEntriesHandler(pool dbPool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid id")
			return
		}
		entries, err := queries.GetAllowlistEntries(r.Context(), pool, id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if entries == nil {
			entries = []models.AllowlistEntry{}
		}
		writeJSON(w, http.StatusOK, entries)
	}
}

func UpdateAllowlistEntryHandler(pool dbPool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entryIDStr := chi.URLParam(r, "entry_id")
		entryID, err := strconv.ParseInt(entryIDStr, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid entry_id")
			return
		}
		var req AddAllowlistEntryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if err := validation.ScopeValue(req.Scope, req.Value); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := queries.UpdateAllowlistEntry(r.Context(), pool, entryID, req.Scope, req.Value, req.Comment); err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"message": "entry updated"})
	}
}

func DeleteAllowlistEntryHandler(pool dbPool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entryIDStr := chi.URLParam(r, "entry_id")
		entryID, err := strconv.ParseInt(entryIDStr, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid entry_id")
			return
		}
		if err := queries.DeleteAllowlistEntry(r.Context(), pool, entryID); err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func AddAllowlistEntryHandler(pool dbPool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid id")
			return
		}
		var req AddAllowlistEntryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if err := validation.ScopeValue(req.Scope, req.Value); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := queries.AddAllowlistEntry(r.Context(), pool, id, req.Scope, req.Value, req.Comment); err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"message": "entry added"})
	}
}
