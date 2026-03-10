package admin

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/aarnaud/crowdsec-central-api/internal/db/queries"
	"github.com/aarnaud/crowdsec-central-api/internal/models"
)

type dbPool = *pgxpool.Pool

func ListMachinesHandler(pool dbPool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		machines, err := queries.ListMachines(r.Context(), pool)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if machines == nil {
			machines = []*models.Machine{}
		}
		writeJSON(w, http.StatusOK, machines)
	}
}

func BlockMachineHandler(pool dbPool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		machineID := chi.URLParam(r, "machine_id")
		if err := queries.UpdateMachineStatus(r.Context(), pool, machineID, "blocked"); err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"message": "machine blocked"})
	}
}

func UnblockMachineHandler(pool dbPool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		machineID := chi.URLParam(r, "machine_id")
		if err := queries.UpdateMachineStatus(r.Context(), pool, machineID, "validated"); err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"message": "machine unblocked"})
	}
}

func DeleteMachineHandler(pool dbPool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		machineID := chi.URLParam(r, "machine_id")
		if err := queries.DeleteMachine(r.Context(), pool, machineID); err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"message": msg})
}
