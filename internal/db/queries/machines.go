package queries

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/aarnaud/crowdsec-central-api/internal/models"
)

func CreateMachine(ctx context.Context, db *pgxpool.Pool, machineID, passwordHash string, ip string) error {
	_, err := db.Exec(ctx, `
		INSERT INTO machines (machine_id, password_hash, ip_address, status)
		VALUES ($1, $2, $3::inet, 'pending')
		ON CONFLICT (machine_id) DO NOTHING
	`, machineID, passwordHash, ip)
	return err
}

func GetMachineByID(ctx context.Context, db *pgxpool.Pool, machineID string) (*models.Machine, error) {
	row := db.QueryRow(ctx, `
		SELECT id, machine_id, password_hash, name, tags, scenarios,
		       ip_address::text, status, enrolled_at, last_seen_at, created_at, updated_at
		FROM machines WHERE machine_id = $1
	`, machineID)

	m := &models.Machine{}
	var ip *string
	err := row.Scan(
		&m.ID, &m.MachineID, &m.PasswordHash, &m.Name, &m.Tags, &m.Scenarios,
		&ip, &m.Status, &m.EnrolledAt, &m.LastSeenAt, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	m.IPAddress = ip
	return m, nil
}

func UpdateMachineLastSeen(ctx context.Context, db *pgxpool.Pool, machineID string) error {
	_, err := db.Exec(ctx, `
		UPDATE machines SET last_seen_at = NOW(), updated_at = NOW()
		WHERE machine_id = $1
	`, machineID)
	return err
}

func EnrollMachine(ctx context.Context, db *pgxpool.Pool, machineID, name string, tags []string) error {
	_, err := db.Exec(ctx, `
		UPDATE machines
		SET status = 'validated', name = $2, tags = $3, enrolled_at = NOW(), updated_at = NOW()
		WHERE machine_id = $1
	`, machineID, name, tags)
	return err
}

func UpdateMachinePassword(ctx context.Context, db *pgxpool.Pool, machineID, passwordHash string) error {
	_, err := db.Exec(ctx, `
		UPDATE machines SET password_hash = $2, updated_at = NOW()
		WHERE machine_id = $1
	`, machineID, passwordHash)
	return err
}

func DeleteMachine(ctx context.Context, db *pgxpool.Pool, machineID string) error {
	_, err := db.Exec(ctx, `DELETE FROM machines WHERE machine_id = $1`, machineID)
	return err
}

func ListMachines(ctx context.Context, db *pgxpool.Pool, limit, offset int) ([]*models.Machine, error) {
	rows, err := db.Query(ctx, `
		SELECT id, machine_id, password_hash, name, tags, scenarios,
		       ip_address::text, status, enrolled_at, last_seen_at, created_at, updated_at
		FROM machines ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var machines []*models.Machine
	for rows.Next() {
		m := &models.Machine{}
		var ip *string
		if err := rows.Scan(
			&m.ID, &m.MachineID, &m.PasswordHash, &m.Name, &m.Tags, &m.Scenarios,
			&ip, &m.Status, &m.EnrolledAt, &m.LastSeenAt, &m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, err
		}
		m.IPAddress = ip
		machines = append(machines, m)
	}
	return machines, rows.Err()
}

var validMachineStatuses = map[string]bool{"pending": true, "validated": true, "blocked": true}

func UpdateMachineStatus(ctx context.Context, db *pgxpool.Pool, machineID, status string) error {
	if !validMachineStatuses[status] {
		return fmt.Errorf("invalid status %q: must be pending, validated, or blocked", status)
	}
	_, err := db.Exec(ctx, `
		UPDATE machines SET status = $2, updated_at = NOW() WHERE machine_id = $1
	`, machineID, status)
	return err
}

func UpdateMachineScenarios(ctx context.Context, db *pgxpool.Pool, machineID string, scenariosJSON []byte) error {
	_, err := db.Exec(ctx, `
		UPDATE machines SET scenarios = $2, updated_at = NOW() WHERE machine_id = $1
	`, machineID, scenariosJSON)
	return err
}

func GetEnrollmentKey(ctx context.Context, db *pgxpool.Pool, key string) error {
	// Atomic check-and-increment: only succeeds if key exists, is not expired,
	// and has not exceeded max_uses. This eliminates the TOCTOU race.
	var id int64
	err := db.QueryRow(ctx, `
		UPDATE enrollment_keys
		SET use_count = use_count + 1
		WHERE key = $1
		  AND (expires_at IS NULL OR expires_at > NOW())
		  AND (max_uses IS NULL OR use_count < max_uses)
		RETURNING id
	`, key).Scan(&id)
	if err != nil {
		// Could be not found, expired, or exhausted — don't leak which
		return fmt.Errorf("invalid or exhausted enrollment key")
	}
	return nil
}
