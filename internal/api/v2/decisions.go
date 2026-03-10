package v2

import (
	"net/http"

	"github.com/rs/zerolog/log"

	"github.com/aarnaud/crowdsec-central-api/internal/auth"
	"github.com/aarnaud/crowdsec-central-api/internal/db/queries"
	"github.com/aarnaud/crowdsec-central-api/internal/models"
)

func DecisionStreamHandler(pool dbPool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		machineID := auth.GetMachineID(r.Context())
		startup := r.URL.Query().Get("startup") == "true"

		newDecs, deletedDecs, err := queries.GetDecisionStream(r.Context(), pool, machineID, startup)
		if err != nil {
			log.Error().Err(err).Str("machine_id", machineID).Msg("getting decision stream")
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		resp := models.DecisionStreamResponse{
			New:     toWireDecisions(newDecs),
			Deleted: toWireDecisions(deletedDecs),
		}
		if resp.New == nil {
			resp.New = []models.DecisionWire{}
		}
		if resp.Deleted == nil {
			resp.Deleted = []models.DecisionWire{}
		}

		writeJSON(w, http.StatusOK, resp)
	}
}

func toWireDecisions(decs []models.Decision) []models.DecisionWire {
	result := make([]models.DecisionWire, 0, len(decs))
	for _, d := range decs {
		wire := models.DecisionWire{
			ID:        d.ID,
			UUID:      d.UUID,
			Origin:    d.Origin,
			Type:      d.Type,
			Scope:     d.Scope,
			Value:     d.Value,
			Duration:  d.Duration.String(),
			Simulated: d.Simulated,
		}
		if d.Scenario != nil {
			wire.Scenario = *d.Scenario
		}
		result = append(result, wire)
	}
	return result
}

func DecisionSyncHandler(pool dbPool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		machineID := auth.GetMachineID(r.Context())
		_ = machineID
		// Accept sync push (for future use / compat)
		w.WriteHeader(http.StatusOK)
	}
}
