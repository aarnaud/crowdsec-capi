package v2

import (
	"encoding/json"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/rs/zerolog/log"

	"github.com/aarnaud/crowdsec-central-api/internal/auth"
	"github.com/aarnaud/crowdsec-central-api/internal/db/queries"
	"github.com/aarnaud/crowdsec-central-api/internal/models"
)

var validate = validator.New()

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"message": msg})
}

// RegisterHandler handles POST /v2/watchers
func RegisterHandler(pool dbPool, jwtMgr *auth.JWTManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req models.RegisterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if err := validate.Struct(req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			log.Error().Err(err).Msg("hashing password")
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		ip := r.RemoteAddr
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			ip = xff
		}

		if err := queries.CreateMachine(r.Context(), pool, req.MachineID, hash, ip); err != nil {
			writeError(w, http.StatusConflict, "machine already exists")
			return
		}

		writeJSON(w, http.StatusCreated, map[string]string{"message": "machine registered"})
	}
}

// LoginHandler handles POST /v2/watchers/login
func LoginHandler(pool dbPool, jwtMgr *auth.JWTManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req models.LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		machine, err := queries.GetMachineByID(r.Context(), pool, req.MachineID)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}

		if !auth.CheckPassword(req.Password, machine.PasswordHash) {
			writeError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}

		// Update scenarios if provided
		if len(req.Scenarios) > 0 {
			scenariosJSON, _ := json.Marshal(req.Scenarios)
			_ = queries.UpdateMachineScenarios(r.Context(), pool, req.MachineID, scenariosJSON)
		}

		_ = queries.UpdateMachineLastSeen(r.Context(), pool, req.MachineID)

		token, exp, err := jwtMgr.Sign(req.MachineID)
		if err != nil {
			log.Error().Err(err).Msg("signing JWT")
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		writeJSON(w, http.StatusOK, models.LoginResponse{
			Code:   http.StatusOK,
			Expire: exp.Format("2006-01-02T15:04:05Z07:00"),
			Token:  token,
		})
	}
}

// EnrollHandler handles POST /v2/watchers/enroll
func EnrollHandler(pool dbPool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		machineID := auth.GetMachineID(r.Context())

		var req models.EnrollRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if err := queries.GetEnrollmentKey(r.Context(), pool, req.EnrollmentKey); err != nil {
			writeError(w, http.StatusForbidden, "invalid enrollment key")
			return
		}

		tags := req.Tags
		if tags == nil {
			tags = []string{}
		}
		if err := queries.EnrollMachine(r.Context(), pool, machineID, req.Name, tags); err != nil {
			log.Error().Err(err).Msg("enrolling machine")
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"message": "machine enrolled"})
	}
}

// ResetPasswordHandler handles POST /v2/watchers/reset
func ResetPasswordHandler(pool dbPool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		machineID := auth.GetMachineID(r.Context())

		var req models.ResetPasswordRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		if err := queries.UpdateMachinePassword(r.Context(), pool, machineID, hash); err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"message": "password reset"})
	}
}

// DeleteSelfHandler handles DELETE /v2/watchers/self
func DeleteSelfHandler(pool dbPool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		machineID := auth.GetMachineID(r.Context())
		if err := queries.DeleteMachine(r.Context(), pool, machineID); err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
