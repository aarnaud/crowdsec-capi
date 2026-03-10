package v2

import (
	"net/http"

	"github.com/aarnaud/crowdsec-central-api/internal/db/queries"
	"github.com/aarnaud/crowdsec-central-api/internal/models"
)

func AllowlistsGetHandler(pool dbPool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		lists, err := queries.GetAllowlists(r.Context(), pool)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if lists == nil {
			lists = []models.Allowlist{}
		}
		writeJSON(w, http.StatusOK, lists)
	}
}

func AllowlistsHeadHandler(pool dbPool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}
}

func AllowlistsPostHandler(pool dbPool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}
}
