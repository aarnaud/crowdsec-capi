package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/aarnaud/crowdsec-central-api/internal/db/queries"
	"github.com/aarnaud/crowdsec-central-api/internal/models"
	"github.com/aarnaud/crowdsec-central-api/internal/validation"
)

const maxSignalBatch = 250

type SignalService struct {
	db              *pgxpool.Pool
	defaultDuration time.Duration
}

func NewSignalService(db *pgxpool.Pool, defaultDuration time.Duration) *SignalService {
	return &SignalService{db: db, defaultDuration: defaultDuration}
}

func (s *SignalService) ProcessSignals(ctx context.Context, machineID string, signals []models.SignalItem) error {
	if len(signals) > maxSignalBatch {
		return fmt.Errorf("batch too large: max %d signals", maxSignalBatch)
	}

	for _, sig := range signals {
		// Parse times
		var startAt, stopAt *time.Time
		if sig.StartAt != "" {
			t, err := time.Parse(time.RFC3339, sig.StartAt)
			if err == nil {
				startAt = &t
			}
		}
		if sig.StopAt != "" {
			t, err := time.Parse(time.RFC3339, sig.StopAt)
			if err == nil {
				stopAt = &t
			}
		}

		labelsJSON, _ := json.Marshal(sig.Labels)

		srcIP := sig.Source.IP
		if srcIP == "" {
			srcIP = sig.Source.Value
		}

		dbSignal := &models.Signal{
			MachineID:       machineID,
			Scenario:        sig.Scenario,
			ScenarioHash:    nilStr(sig.ScenarioHash),
			ScenarioVersion: nilStr(sig.ScenarioVersion),
			SourceScope:     nilStr(sig.Source.Scope),
			SourceValue:     nilStr(sig.Source.Value),
			SourceIP:        nilStr(srcIP),
			Labels:          labelsJSON,
			StartAt:         startAt,
			StopAt:          stopAt,
			AlertCount:      sig.AlertCount,
		}
		if err := queries.CreateSignal(ctx, s.db, dbSignal); err != nil {
			// Non-fatal, continue processing
			continue
		}

		// Process decisions from signal
		for _, dec := range sig.Decisions {
			if dec.Scope == "" || dec.Value == "" {
				continue
			}
			if err := validation.DecisionFields(dec.Type, dec.Scope, dec.Value); err != nil {
				continue
			}
			// Check allowlist
			allowed, err := queries.IsAllowlisted(ctx, s.db, dec.Scope, dec.Value)
			if err != nil || allowed {
				continue
			}
			// Dedup: skip if unexpired decision exists
			existing, _ := queries.GetActiveDecision(ctx, s.db, dec.Scope, dec.Value)
			if existing != nil {
				continue
			}
			// Create decision
			duration := s.defaultDuration
			if dec.Duration != "" {
				if d, err := time.ParseDuration(dec.Duration); err == nil {
					duration = d
				}
			}
			scenario := sig.Scenario
			dbDec := &models.Decision{
				Origin:          "local-signal",
				Type:            dec.Type,
				Scope:           dec.Scope,
				Value:           dec.Value,
				Duration:        duration,
				Scenario:        &scenario,
				SourceMachineID: &machineID,
				Simulated:       false,
				ExpiresAt:       time.Now().Add(duration),
			}
			_ = queries.CreateDecision(ctx, s.db, dbDec)
		}
	}
	return nil
}

func nilStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
