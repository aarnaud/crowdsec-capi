package admin

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

func UpstreamStatusHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		type State struct {
			LastSyncAt    *time.Time `json:"last_sync_at"`
			LastStartupAt *time.Time `json:"last_startup_at"`
			NextSyncAt    *time.Time `json:"next_sync_at"`
			EnrolledAt    *time.Time `json:"enrolled_at"`
			MachineID     *string    `json:"machine_id"`
			DecisionCount int        `json:"decision_count"`
		}
		var s State
		err := pool.QueryRow(r.Context(), `
			SELECT last_sync_at, last_startup_at, enrolled_at, machine_id, decision_count
			FROM upstream_sync_state WHERE id = 1
		`).Scan(&s.LastSyncAt, &s.LastStartupAt, &s.EnrolledAt, &s.MachineID, &s.DecisionCount)
		if err != nil {
			// No row means upstream sync has never run — return an empty state.
			if err == pgx.ErrNoRows {
				writeJSON(w, http.StatusOK, s)
				return
			}
			log.Error().Err(err).Msg("upstream status: query failed")
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if s.LastSyncAt != nil {
			next := s.LastSyncAt.Add(2 * time.Hour)
			s.NextSyncAt = &next
		}
		writeJSON(w, http.StatusOK, s)
	}
}
