package v2

import (
	"net/http"
	"strconv"
	"time"

	"github.com/aarnaud/crowdsec-central-api/internal/auth"
)

func PAPIHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		machineID := auth.GetMachineID(r.Context())
		_ = machineID
		// Stub: return empty orders
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"links": map[string]interface{}{},
			"items": []interface{}{},
		})
	}
}

// PAPIPermissionsHandler handles GET /v1/permissions
func PAPIPermissionsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":     "ok",
			"plan":       "self-hosted",
			"categories": []string{"decisions"},
		})
	}
}

// PAPIPollHandler handles GET /v1/decisions/stream/poll
// It holds the connection for the requested timeout then signals no events.
func PAPIPollHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		timeoutSecs := 30
		if t := r.URL.Query().Get("timeout"); t != "" {
			if v, err := strconv.Atoi(t); err == nil && v > 0 && v <= 60 {
				timeoutSecs = v
			}
		}

		select {
		case <-time.After(time.Duration(timeoutSecs) * time.Second):
		case <-r.Context().Done():
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"error":     "no events before timeout",
			"timestamp": time.Now().UnixMilli(),
		})
	}
}
