package queries

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// GetOrCreateJWTSecret retrieves the persistent JWT signing secret from the
// database, creating and storing a new one if none exists yet. Using the
// database as the source of truth ensures the secret survives restarts and
// is shared across all replicas.
func GetOrCreateJWTSecret(ctx context.Context, db *pgxpool.Pool) ([]byte, error) {
	// Fast path: secret already exists
	var hexSecret string
	err := db.QueryRow(ctx, `SELECT value FROM server_settings WHERE key = 'jwt_secret'`).Scan(&hexSecret)
	if err == nil {
		return hex.DecodeString(hexSecret)
	}

	// Generate a new 32-byte secret
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("generating JWT secret: %w", err)
	}
	hexSecret = hex.EncodeToString(b)

	// INSERT ... ON CONFLICT DO NOTHING is safe for concurrent replicas:
	// only the first writer wins; all others silently lose the race.
	_, err = db.Exec(ctx,
		`INSERT INTO server_settings (key, value) VALUES ('jwt_secret', $1) ON CONFLICT (key) DO NOTHING`,
		hexSecret,
	)
	if err != nil {
		return nil, fmt.Errorf("storing JWT secret: %w", err)
	}

	// Re-read to get the winner's value in case of concurrent insert
	err = db.QueryRow(ctx, `SELECT value FROM server_settings WHERE key = 'jwt_secret'`).Scan(&hexSecret)
	if err != nil {
		return nil, fmt.Errorf("reading JWT secret: %w", err)
	}
	return hex.DecodeString(hexSecret)
}
