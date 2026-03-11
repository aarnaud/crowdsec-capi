package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/aarnaud/crowdsec-central-api/internal/db/queries"
	"github.com/aarnaud/crowdsec-central-api/internal/models"
	"github.com/aarnaud/crowdsec-central-api/internal/validation"
)

const maxSignalBatch = 250

// ErrMachineBlocked is returned when a blocked machine attempts to submit signals.
var ErrMachineBlocked = errors.New("machine is blocked")

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

	machine, err := queries.GetMachineByID(ctx, s.db, machineID)
	if err != nil {
		return fmt.Errorf("machine not found")
	}
	if machine.Status == "blocked" {
		return ErrMachineBlocked
	}

	for _, sig := range signals {
		// Skip signals with oversized fields to prevent DB bloat
		if len(sig.Scenario) > 1024 {
			continue
		}
		labelsJSON, _ := json.Marshal(sig.Labels)
		if len(labelsJSON) > 4096 {
			continue
		}

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

		// Only store source_ip when it's actually an IP (not username/country/etc.)
		var sourceIP, sourceRange *string
		switch sig.Source.Scope {
		case "Ip", "ip":
			if sig.Source.IP != "" {
				sourceIP = nilStr(sig.Source.IP)
			} else {
				sourceIP = nilStr(sig.Source.Value)
			}
		case "Range", "range":
			sourceRange = nilStr(sig.Source.Range)
			if sourceRange == nil {
				sourceRange = nilStr(sig.Source.Value)
			}
		}

		var asNumber *int
		if sig.Source.AsNumber != "" {
			if n, err := strconv.Atoi(sig.Source.AsNumber); err == nil {
				asNumber = &n
			}
		}
		var lat, lon *float64
		if sig.Source.Latitude != 0 {
			lat = &sig.Source.Latitude
		}
		if sig.Source.Longitude != 0 {
			lon = &sig.Source.Longitude
		}

		dbSignal := &models.Signal{
			MachineID:       machineID,
			Scenario:        sig.Scenario,
			ScenarioHash:    nilStr(sig.ScenarioHash),
			ScenarioVersion: nilStr(sig.ScenarioVersion),
			SourceScope:     nilStr(sig.Source.Scope),
			SourceValue:     nilStr(sig.Source.Value),
			SourceIP:        sourceIP,
			SourceRange:     sourceRange,
			SourceAsName:    nilStr(sig.Source.AsName),
			SourceAsNumber:  asNumber,
			SourceCountry:   nilStr(sig.Source.CN),
			SourceLatitude:  lat,
			SourceLongitude: lon,
			Labels:          labelsJSON,
			StartAt:         startAt,
			StopAt:          stopAt,
			AlertCount:      sig.AlertCount,
		}
		if err := queries.CreateSignal(ctx, s.db, dbSignal); err != nil {
			log.Error().Err(err).Str("machine_id", machineID).Str("scenario", sig.Scenario).Msg("storing signal")
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
