package queries

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/aarnaud/crowdsec-central-api/internal/models"
)

func IsAllowlisted(ctx context.Context, db *pgxpool.Pool, scope, value string) (bool, error) {
	var count int
	err := db.QueryRow(ctx, `
		SELECT COUNT(*) FROM allowlist_entries
		WHERE scope = $1 AND value = $2
		  AND (expires_at IS NULL OR expires_at > NOW())
	`, scope, value).Scan(&count)
	return count > 0, err
}

// GetAllowlistByName returns an allowlist by name, or nil if not found.
func GetAllowlistByName(ctx context.Context, db *pgxpool.Pool, name string) (*models.Allowlist, error) {
	a := &models.Allowlist{}
	err := db.QueryRow(ctx, `
		SELECT id, name, label, description, managed, created_at, updated_at
		FROM allowlists WHERE name = $1
	`, name).Scan(&a.ID, &a.Name, &a.Label, &a.Description, &a.Managed, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return a, nil
}

// CheckAllowlistedValue checks if a value (IP or range) is in any allowlist.
// Returns the matching allowlist name and entry comment, or empty strings if not found.
func CheckAllowlistedValue(ctx context.Context, db *pgxpool.Pool, value string) (string, string, error) {
	var allowlistName, comment string
	err := db.QueryRow(ctx, `
		SELECT a.name, COALESCE(e.comment, '')
		FROM allowlist_entries e
		JOIN allowlists a ON a.id = e.allowlist_id
		WHERE e.value = $1
		  AND (e.expires_at IS NULL OR e.expires_at > NOW())
		LIMIT 1
	`, value).Scan(&allowlistName, &comment)
	if err != nil {
		return "", "", err // pgx.ErrNoRows if not found
	}
	return allowlistName, comment, nil
}

func GetAllowlists(ctx context.Context, db *pgxpool.Pool) ([]models.Allowlist, error) {
	rows, err := db.Query(ctx, `
		SELECT id, name, label, description, managed, created_at, updated_at
		FROM allowlists ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lists []models.Allowlist
	for rows.Next() {
		var a models.Allowlist
		if err := rows.Scan(&a.ID, &a.Name, &a.Label, &a.Description, &a.Managed, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		lists = append(lists, a)
	}
	return lists, rows.Err()
}

func CreateAllowlist(ctx context.Context, db *pgxpool.Pool, name, label, description string) (*models.Allowlist, error) {
	a := &models.Allowlist{}
	err := db.QueryRow(ctx, `
		INSERT INTO allowlists (name, label, description)
		VALUES ($1, $2, $3)
		RETURNING id, name, label, description, managed, created_at, updated_at
	`, name, label, description).Scan(&a.ID, &a.Name, &a.Label, &a.Description, &a.Managed, &a.CreatedAt, &a.UpdatedAt)
	return a, err
}

// UpsertManagedAllowlist creates or updates an allowlist and marks it as managed.
// It returns the allowlist ID.
func UpsertManagedAllowlist(ctx context.Context, db *pgxpool.Pool, name, label, description string) (int64, error) {
	var id int64
	err := db.QueryRow(ctx, `
		INSERT INTO allowlists (name, label, description, managed)
		VALUES ($1, $2, $3, TRUE)
		ON CONFLICT (name) DO UPDATE SET
			label       = EXCLUDED.label,
			description = EXCLUDED.description,
			managed     = TRUE,
			updated_at  = NOW()
		RETURNING id
	`, name, label, description).Scan(&id)
	return id, err
}

// SyncManagedEntries replaces all entries for a managed allowlist with the provided set.
func SyncManagedEntries(ctx context.Context, db *pgxpool.Pool, allowlistID int64, entries []models.AllowlistEntry) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Delete entries not present in the new set (keep manually-added ones that
	// are not in the file by only deleting those whose comment matches managed marker).
	// Strategy: delete all then re-insert — managed file is the source of truth.
	if _, err := tx.Exec(ctx, `DELETE FROM allowlist_entries WHERE allowlist_id = $1`, allowlistID); err != nil {
		return fmt.Errorf("clearing entries: %w", err)
	}

	for _, e := range entries {
		if _, err := tx.Exec(ctx, `
			INSERT INTO allowlist_entries (allowlist_id, scope, value, comment)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (allowlist_id, scope, value) DO UPDATE SET comment = EXCLUDED.comment
		`, allowlistID, e.Scope, e.Value, e.Comment); err != nil {
			return fmt.Errorf("inserting entry %s/%s: %w", e.Scope, e.Value, err)
		}
	}

	return tx.Commit(ctx)
}

func DeleteAllowlist(ctx context.Context, db *pgxpool.Pool, id int64) error {
	var managed bool
	if err := db.QueryRow(ctx, `SELECT managed FROM allowlists WHERE id = $1`, id).Scan(&managed); err != nil {
		return fmt.Errorf("allowlist not found")
	}
	if managed {
		return fmt.Errorf("cannot delete a managed allowlist (remove it from the allowlists file)")
	}
	_, err := db.Exec(ctx, `DELETE FROM allowlists WHERE id = $1`, id)
	return err
}

func GetAllowlistEntries(ctx context.Context, db *pgxpool.Pool, allowlistID int64) ([]models.AllowlistEntry, error) {
	rows, err := db.Query(ctx, `
		SELECT id, allowlist_id, scope, value, comment, expires_at, created_at
		FROM allowlist_entries WHERE allowlist_id = $1 ORDER BY created_at
	`, allowlistID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []models.AllowlistEntry
	for rows.Next() {
		var e models.AllowlistEntry
		if err := rows.Scan(&e.ID, &e.AllowlistID, &e.Scope, &e.Value, &e.Comment, &e.ExpiresAt, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func AddAllowlistEntry(ctx context.Context, db *pgxpool.Pool, allowlistID int64, scope, value, comment string) error {
	_, err := db.Exec(ctx, `
		INSERT INTO allowlist_entries (allowlist_id, scope, value, comment)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (allowlist_id, scope, value) DO NOTHING
	`, allowlistID, scope, value, comment)
	return err
}

func UpdateAllowlistEntry(ctx context.Context, db *pgxpool.Pool, entryID int64, scope, value, comment string) error {
	_, err := db.Exec(ctx, `
		UPDATE allowlist_entries SET scope = $2, value = $3, comment = $4 WHERE id = $1
	`, entryID, scope, value, comment)
	return err
}

func DeleteAllowlistEntry(ctx context.Context, db *pgxpool.Pool, entryID int64) error {
	_, err := db.Exec(ctx, `DELETE FROM allowlist_entries WHERE id = $1`, entryID)
	return err
}
