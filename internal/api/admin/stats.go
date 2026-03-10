package admin

import (
	"net/http"

	"github.com/aarnaud/crowdsec-central-api/internal/db/queries"
)

func StatsHandler(pool dbPool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stats, err := queries.GetStats(r.Context(), pool)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		writeJSON(w, http.StatusOK, stats)
	}
}
