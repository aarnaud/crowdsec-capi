package admin

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func UpstreamStatusHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		type State struct {
			LastSyncAt    *time.Time `json:"last_sync_at"`
			MachineID     *string    `json:"machine_id"`
			DecisionCount int        `json:"decision_count"`
		}
		var s State
		err := pool.QueryRow(r.Context(), `
			SELECT last_sync_at, machine_id, decision_count FROM upstream_sync_state WHERE id = 1
		`).Scan(&s.LastSyncAt, &s.MachineID, &s.DecisionCount)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		writeJSON(w, http.StatusOK, s)
	}
}
