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
