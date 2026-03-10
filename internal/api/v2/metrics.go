package v2

import (
	"net/http"

	"github.com/aarnaud/crowdsec-central-api/internal/auth"
	"github.com/aarnaud/crowdsec-central-api/internal/db/queries"
)

func MetricsHandler(pool dbPool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		machineID := auth.GetMachineID(r.Context())
		_ = queries.UpdateMachineLastSeen(r.Context(), pool, machineID)
		w.WriteHeader(http.StatusOK)
	}
}

func HeartbeatHandler(pool dbPool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		machineID := auth.GetMachineID(r.Context())
		_ = queries.UpdateMachineLastSeen(r.Context(), pool, machineID)
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}
