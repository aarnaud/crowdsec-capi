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
		INSERT INTO signals (machine_id, scenario, scenario_hash, scenario_version,
			source_scope, source_value, source_ip, labels, start_at, stop_at, alert_count)
		VALUES ($1, $2, $3, $4, $5, $6, $7::inet, $8, $9, $10, $11)
	`, s.MachineID, s.Scenario, s.ScenarioHash, s.ScenarioVersion,
		s.SourceScope, s.SourceValue, s.SourceIP, labelsJSON,
		s.StartAt, s.StopAt, s.AlertCount)
	return err
}
