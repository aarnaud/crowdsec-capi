package admin

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ListEnrollmentKeysHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := pool.Query(r.Context(), `
			SELECT id, key, description, tags, max_uses, use_count, expires_at, created_at
			FROM enrollment_keys ORDER BY created_at DESC
		`)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		defer rows.Close()

		type Key struct {
			ID          int64      `json:"id"`
			Key         string     `json:"key"`
			Description *string    `json:"description"`
			Tags        []string   `json:"tags"`
			MaxUses     *int       `json:"max_uses"`
			UseCount    int        `json:"use_count"`
			ExpiresAt   *time.Time `json:"expires_at"`
			CreatedAt   time.Time  `json:"created_at"`
		}
		var keys []Key
		for rows.Next() {
			var k Key
			if err := rows.Scan(&k.ID, &k.Key, &k.Description, &k.Tags, &k.MaxUses, &k.UseCount, &k.ExpiresAt, &k.CreatedAt); err != nil {
				writeError(w, http.StatusInternalServerError, "internal error")
				return
			}
			// Truncate the key so the full value is only visible at creation time
			if len(k.Key) > 8 {
				k.Key = k.Key[:8] + "..."
			}
			keys = append(keys, k)
		}
		if keys == nil {
			keys = []Key{}
		}
		writeJSON(w, http.StatusOK, keys)
	}
}

type CreateEnrollmentKeyRequest struct {
	Description string     `json:"description"`
	Tags        []string   `json:"tags"`
	MaxUses     *int       `json:"max_uses"`
	ExpiresAt   *time.Time `json:"expires_at"`
}

func CreateEnrollmentKeyHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CreateEnrollmentKeyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.MaxUses != nil && *req.MaxUses < 1 {
			writeError(w, http.StatusBadRequest, "max_uses must be ≥ 1")
			return
		}
		if req.ExpiresAt != nil && !req.ExpiresAt.After(time.Now()) {
			writeError(w, http.StatusBadRequest, "expires_at must be in the future")
			return
		}

		keyBytes := make([]byte, 32)
		if _, err := rand.Read(keyBytes); err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		key := hex.EncodeToString(keyBytes)

		if req.Tags == nil {
			req.Tags = []string{}
		}

		var id int64
		var createdAt time.Time
		err := pool.QueryRow(r.Context(), `
			INSERT INTO enrollment_keys (key, description, tags, max_uses, expires_at)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id, created_at
		`, key, req.Description, req.Tags, req.MaxUses, req.ExpiresAt).Scan(&id, &createdAt)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		writeJSON(w, http.StatusCreated, map[string]interface{}{
			"id":          id,
			"key":         key,
			"description": req.Description,
			"tags":        req.Tags,
			"max_uses":    req.MaxUses,
			"expires_at":  req.ExpiresAt,
			"created_at":  createdAt,
		})
	}
}

func DeleteEnrollmentKeyHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		_, err := pool.Exec(r.Context(), `DELETE FROM enrollment_keys WHERE id = $1`, id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
