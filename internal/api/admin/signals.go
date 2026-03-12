package admin

import (
	"net/http"

	"github.com/aarnaud/crowdsec-central-api/internal/db/queries"
	"github.com/aarnaud/crowdsec-central-api/internal/models"
)

func ListSignalsHandler(pool dbPool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, offset := parsePagination(r)
		signals, err := queries.ListSignals(r.Context(), pool, limit, offset)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if signals == nil {
			signals = []models.Signal{}
		}
		writeJSON(w, http.StatusOK, signals)
	}
}
