package v2

import (
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog/log"

	"github.com/aarnaud/crowdsec-central-api/internal/auth"
	"github.com/aarnaud/crowdsec-central-api/internal/models"
	"github.com/aarnaud/crowdsec-central-api/internal/service"
)

func SignalsHandler(svc *service.SignalService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		machineID := auth.GetMachineID(r.Context())

		var signals []models.SignalItem
		if err := json.NewDecoder(r.Body).Decode(&signals); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if err := svc.ProcessSignals(r.Context(), machineID, signals); err != nil {
			log.Error().Err(err).Str("machine_id", machineID).Msg("processing signals")
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}
