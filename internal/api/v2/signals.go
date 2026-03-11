package v2

import (
	"encoding/json"
	"errors"
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
			log.Error().Err(err).
				Str("machine_id", machineID).
				Str("content_type", r.Header.Get("Content-Type")).
				Msg("signals: failed to decode request body")
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		log.Debug().Str("machine_id", machineID).Int("count", len(signals)).Msg("signals received")

		if err := svc.ProcessSignals(r.Context(), machineID, signals); err != nil {
			if errors.Is(err, service.ErrMachineBlocked) {
				writeError(w, http.StatusForbidden, "machine is blocked")
				return
			}
			log.Error().Err(err).Str("machine_id", machineID).Msg("signals: processing failed")
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}
