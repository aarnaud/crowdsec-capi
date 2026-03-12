package queries

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/aarnaud/crowdsec-central-api/internal/models"
)

func CreateSignal(ctx context.Context, db *pgxpool.Pool, s *models.Signal) error {
	labelsJSON, _ := json.Marshal(s.Labels)
	_, err := db.Exec(ctx, `
		INSERT INTO signals (
			machine_id, scenario, scenario_hash, scenario_version,
			source_scope, source_value,
			source_ip, source_range,
			source_as_name, source_as_number,
			source_country, source_latitude, source_longitude,
			labels, start_at, stop_at, alert_count
		) VALUES (
			$1, $2, $3, $4,
			$5, $6,
			NULLIF($7, '')::inet, NULLIF($8, '')::cidr,
			$9, $10,
			$11, $12, $13,
			$14, $15, $16, $17
		)
	`,
		s.MachineID, s.Scenario, s.ScenarioHash, s.ScenarioVersion,
		s.SourceScope, s.SourceValue,
		nilPtrStr(s.SourceIP), nilPtrStr(s.SourceRange),
		s.SourceAsName, s.SourceAsNumber,
		s.SourceCountry, s.SourceLatitude, s.SourceLongitude,
		labelsJSON, s.StartAt, s.StopAt, s.AlertCount,
	)
	return err
}

// nilPtrStr dereferences a *string to a plain string for use with NULLIF in SQL.
// A nil pointer becomes empty string, which NULLIF then converts to NULL.
func nilPtrStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func ListSignals(ctx context.Context, db *pgxpool.Pool, limit, offset int) ([]models.Signal, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := db.Query(ctx, `
		SELECT id, uuid, machine_id, scenario, scenario_hash, scenario_version,
		       source_scope, source_value,
		       COALESCE(source_ip::text, ''), COALESCE(source_range::text, ''),
		       source_as_name, source_as_number,
		       source_country, source_latitude, source_longitude,
		       labels, start_at, stop_at, alert_count, created_at
		FROM signals
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var signals []models.Signal
	for rows.Next() {
		var s models.Signal
		var sourceIP, sourceRange string
		var labelsJSON []byte
		if err := rows.Scan(
			&s.ID, &s.UUID, &s.MachineID, &s.Scenario, &s.ScenarioHash, &s.ScenarioVersion,
			&s.SourceScope, &s.SourceValue,
			&sourceIP, &sourceRange,
			&s.SourceAsName, &s.SourceAsNumber,
			&s.SourceCountry, &s.SourceLatitude, &s.SourceLongitude,
			&labelsJSON, &s.StartAt, &s.StopAt, &s.AlertCount, &s.CreatedAt,
		); err != nil {
			return nil, err
		}
		if sourceIP != "" {
			s.SourceIP = &sourceIP
		}
		if sourceRange != "" {
			s.SourceRange = &sourceRange
		}
		s.Labels = labelsJSON
		signals = append(signals, s)
	}
	return signals, rows.Err()
}
