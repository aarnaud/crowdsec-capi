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

func V3DecisionStreamHandler(pool dbPool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		machineID := auth.GetMachineID(r.Context())
		startup := r.URL.Query().Get("startup") == "true"

		newDecs, deletedDecs, err := queries.GetDecisionStream(r.Context(), pool, machineID, startup)
		if err != nil {
			log.Error().Err(err).Str("machine_id", machineID).Msg("getting v3 decision stream")
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		// Group new decisions by scenario+scope
		type newKey struct{ scenario, scope string }
		newGroups := map[newKey]*models.V3DecisionNewGroup{}
		var newOrder []newKey
		for _, d := range newDecs {
			scenario := ""
			if d.Scenario != nil {
				scenario = *d.Scenario
			}
			k := newKey{scenario, d.Scope}
			if _, ok := newGroups[k]; !ok {
				newGroups[k] = &models.V3DecisionNewGroup{Scenario: scenario, Scope: d.Scope, Decisions: []models.V3DecisionNew{}}
				newOrder = append(newOrder, k)
			}
			newGroups[k].Decisions = append(newGroups[k].Decisions, models.V3DecisionNew{
				Duration: d.Duration.String(),
				Value:    d.Value,
			})
		}
		newList := make([]models.V3DecisionNewGroup, 0, len(newOrder))
		for _, k := range newOrder {
			newList = append(newList, *newGroups[k])
		}

		// Group deleted decisions by scope
		deletedGroups := map[string]*models.V3DecisionDeletedGroup{}
		var deletedOrder []string
		for _, d := range deletedDecs {
			if _, ok := deletedGroups[d.Scope]; !ok {
				deletedGroups[d.Scope] = &models.V3DecisionDeletedGroup{Scope: d.Scope, Decisions: []string{}}
				deletedOrder = append(deletedOrder, d.Scope)
			}
			deletedGroups[d.Scope].Decisions = append(deletedGroups[d.Scope].Decisions, d.Value)
		}
		deletedList := make([]models.V3DecisionDeletedGroup, 0, len(deletedOrder))
		for _, k := range deletedOrder {
			deletedList = append(deletedList, *deletedGroups[k])
		}

		writeJSON(w, http.StatusOK, models.V3DecisionStreamResponse{
			New:     newList,
			Deleted: deletedList,
		})
	}
}

func DecisionSyncHandler(pool dbPool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		machineID := auth.GetMachineID(r.Context())
		_ = machineID
		// Accept sync push (for future use / compat)
		w.WriteHeader(http.StatusOK)
	}
}
