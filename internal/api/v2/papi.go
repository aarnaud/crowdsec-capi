package v2

import (
	"net/http"

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
