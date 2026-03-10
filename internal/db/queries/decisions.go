package queries

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/aarnaud/crowdsec-central-api/internal/models"
)

func CreateDecision(ctx context.Context, db *pgxpool.Pool, d *models.Decision) error {
	durationStr := fmt.Sprintf("%d seconds", int(d.Duration.Seconds()))
	_, err := db.Exec(ctx, `
		INSERT INTO decisions (origin, type, scope, value, duration, scenario, source_machine_id, simulated, expires_at)
		VALUES ($1, $2, $3, $4, $5::interval, $6, $7, $8, $9)
	`, d.Origin, d.Type, d.Scope, d.Value, durationStr, d.Scenario, d.SourceMachineID, d.Simulated, d.ExpiresAt)
	return err
}

func GetActiveDecision(ctx context.Context, db *pgxpool.Pool, scope, value string) (*models.Decision, error) {
	row := db.QueryRow(ctx, `
		SELECT id, uuid, origin, type, scope, value,
		       EXTRACT(EPOCH FROM duration)::bigint as duration_secs,
		       scenario, source_machine_id, simulated, is_deleted, expires_at, created_at, updated_at, deleted_at
		FROM decisions
		WHERE scope = $1 AND value = $2 AND is_deleted = FALSE AND expires_at > NOW()
		ORDER BY created_at DESC LIMIT 1
	`, scope, value)

	d := &models.Decision{}
	var durationSecs int64
	err := row.Scan(
		&d.ID, &d.UUID, &d.Origin, &d.Type, &d.Scope, &d.Value,
		&durationSecs, &d.Scenario, &d.SourceMachineID, &d.Simulated, &d.IsDeleted,
		&d.ExpiresAt, &d.CreatedAt, &d.UpdatedAt, &d.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	d.Duration = time.Duration(durationSecs) * time.Second
	return d, nil
}

func GetDecisionStream(ctx context.Context, db *pgxpool.Pool, machineID string, startup bool) ([]models.Decision, []models.Decision, error) {
	var cursor time.Time

	if !startup {
		err := db.QueryRow(ctx, `
			SELECT last_pulled_at FROM machine_decision_cursors WHERE machine_id = $1
		`, machineID).Scan(&cursor)
		if err != nil {
			// No cursor yet, treat as startup
			startup = true
		}
	}

	var newDecisions, deletedDecisions []models.Decision

	if startup {
		rows, err := db.Query(ctx, `
			SELECT id, uuid, origin, type, scope, value,
			       EXTRACT(EPOCH FROM duration)::bigint,
			       scenario, source_machine_id, simulated, is_deleted, expires_at, created_at, updated_at, deleted_at
			FROM decisions
			WHERE is_deleted = FALSE AND expires_at > NOW()
			ORDER BY created_at ASC
		`)
		if err != nil {
			return nil, nil, err
		}
		defer rows.Close()
		for rows.Next() {
			d, err := scanDecision(rows)
			if err != nil {
				return nil, nil, err
			}
			newDecisions = append(newDecisions, *d)
		}
		if err := rows.Err(); err != nil {
			return nil, nil, err
		}
	} else {
		// New decisions since cursor
		rows, err := db.Query(ctx, `
			SELECT id, uuid, origin, type, scope, value,
			       EXTRACT(EPOCH FROM duration)::bigint,
			       scenario, source_machine_id, simulated, is_deleted, expires_at, created_at, updated_at, deleted_at
			FROM decisions
			WHERE is_deleted = FALSE AND expires_at > NOW() AND updated_at > $1
			ORDER BY updated_at ASC
		`, cursor)
		if err != nil {
			return nil, nil, err
		}
		defer rows.Close()
		for rows.Next() {
			d, err := scanDecision(rows)
			if err != nil {
				return nil, nil, err
			}
			newDecisions = append(newDecisions, *d)
		}
		if err := rows.Err(); err != nil {
			return nil, nil, err
		}

		// Deleted decisions since cursor
		rows2, err := db.Query(ctx, `
			SELECT id, uuid, origin, type, scope, value,
			       EXTRACT(EPOCH FROM duration)::bigint,
			       scenario, source_machine_id, simulated, is_deleted, expires_at, created_at, updated_at, deleted_at
			FROM decisions
			WHERE is_deleted = TRUE AND deleted_at > $1
			ORDER BY deleted_at ASC
		`, cursor)
		if err != nil {
			return nil, nil, err
		}
		defer rows2.Close()
		for rows2.Next() {
			d, err := scanDecision(rows2)
			if err != nil {
				return nil, nil, err
			}
			deletedDecisions = append(deletedDecisions, *d)
		}
		if err := rows2.Err(); err != nil {
			return nil, nil, err
		}
	}

	// Update cursor
	_, err := db.Exec(ctx, `
		INSERT INTO machine_decision_cursors (machine_id, last_pulled_at)
		VALUES ($1, NOW())
		ON CONFLICT (machine_id) DO UPDATE SET last_pulled_at = NOW()
	`, machineID)
	if err != nil {
		return nil, nil, fmt.Errorf("updating cursor: %w", err)
	}

	return newDecisions, deletedDecisions, nil
}

type scannable interface {
	Scan(dest ...any) error
}

func scanDecision(row scannable) (*models.Decision, error) {
	d := &models.Decision{}
	var durationSecs int64
	err := row.Scan(
		&d.ID, &d.UUID, &d.Origin, &d.Type, &d.Scope, &d.Value,
		&durationSecs, &d.Scenario, &d.SourceMachineID, &d.Simulated, &d.IsDeleted,
		&d.ExpiresAt, &d.CreatedAt, &d.UpdatedAt, &d.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	d.Duration = time.Duration(durationSecs) * time.Second
	return d, nil
}

func SoftDeleteExpiredDecisions(ctx context.Context, db *pgxpool.Pool) (int64, error) {
	tag, err := db.Exec(ctx, `
		UPDATE decisions
		SET is_deleted = TRUE, deleted_at = NOW(), updated_at = NOW()
		WHERE is_deleted = FALSE AND expires_at <= NOW()
	`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func ListDecisions(ctx context.Context, db *pgxpool.Pool, includeDeleted bool) ([]models.Decision, error) {
	query := `
		SELECT id, uuid, origin, type, scope, value,
		       EXTRACT(EPOCH FROM duration)::bigint,
		       scenario, source_machine_id, simulated, is_deleted, expires_at, created_at, updated_at, deleted_at
		FROM decisions
	`
	if !includeDeleted {
		query += " WHERE is_deleted = FALSE AND expires_at > NOW()"
	}
	query += " ORDER BY created_at DESC"

	rows, err := db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var decisions []models.Decision
	for rows.Next() {
		d, err := scanDecision(rows)
		if err != nil {
			return nil, err
		}
		decisions = append(decisions, *d)
	}
	return decisions, rows.Err()
}

func DeleteDecision(ctx context.Context, db *pgxpool.Pool, uuid string) error {
	_, err := db.Exec(ctx, `
		UPDATE decisions
		SET is_deleted = TRUE, deleted_at = NOW(), updated_at = NOW()
		WHERE uuid = $1
	`, uuid)
	return err
}

func UpsertUpstreamDecision(ctx context.Context, db *pgxpool.Pool, d *models.Decision) error {
	durationStr := fmt.Sprintf("%d seconds", int(d.Duration.Seconds()))
	_, err := db.Exec(ctx, `
		INSERT INTO decisions (uuid, origin, type, scope, value, duration, scenario, simulated, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6::interval, $7, $8, $9)
		ON CONFLICT (uuid) DO UPDATE SET
			is_deleted = FALSE,
			deleted_at = NULL,
			expires_at = EXCLUDED.expires_at,
			updated_at = NOW()
	`, d.UUID, d.Origin, d.Type, d.Scope, d.Value, durationStr, d.Scenario, d.Simulated, d.ExpiresAt)
	return err
}

func MarkUpstreamDecisionDeleted(ctx context.Context, db *pgxpool.Pool, uuid string) error {
	_, err := db.Exec(ctx, `
		UPDATE decisions
		SET is_deleted = TRUE, deleted_at = NOW(), updated_at = NOW()
		WHERE uuid = $1 AND origin = 'upstream-capi'
	`, uuid)
	return err
}
