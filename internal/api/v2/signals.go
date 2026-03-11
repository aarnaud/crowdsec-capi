package v2

import (
	"errors"
	"io"
	"net/http"

	"github.com/rs/zerolog/log"

	"github.com/aarnaud/crowdsec-central-api/internal/auth"
	"github.com/aarnaud/crowdsec-central-api/internal/models"
	"github.com/aarnaud/crowdsec-central-api/internal/service"

	"encoding/json"
)

func SignalsHandler(svc *service.SignalService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		machineID := auth.GetMachineID(r.Context())

		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, "cannot read body")
			return
		}
		log.Debug().
			Str("machine_id", machineID).
			Str("content_encoding", r.Header.Get("Content-Encoding")).
			Int("body_len", len(body)).
			Str("body_prefix", truncate(string(body), 200)).
			Msg("signals raw body")

		var signals []models.SignalItem
		if err := json.Unmarshal(body, &signals); err != nil {
			log.Error().Err(err).Str("machine_id", machineID).Msg("signals decode error")
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		log.Debug().Str("machine_id", machineID).Int("count", len(signals)).Msg("signals decoded")

		if err := svc.ProcessSignals(r.Context(), machineID, signals); err != nil {
			if errors.Is(err, service.ErrMachineBlocked) {
				writeError(w, http.StatusForbidden, "machine is blocked")
				return
			}
			log.Error().Err(err).Str("machine_id", machineID).Msg("processing signals")
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
