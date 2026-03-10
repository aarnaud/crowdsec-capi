package upstream

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/aarnaud/crowdsec-central-api/internal/db/queries"
	"github.com/aarnaud/crowdsec-central-api/internal/models"
)

type Syncer struct {
	client   *Client
	db       *pgxpool.Pool
	interval time.Duration
	startup  bool
}

func NewSyncer(client *Client, db *pgxpool.Pool, interval time.Duration) *Syncer {
	return &Syncer{
		client:   client,
		db:       db,
		interval: interval,
		startup:  true,
	}
}

func (s *Syncer) Run(ctx context.Context) {
	log.Info().Msg("upstream syncer started")
	s.sync(ctx)
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.sync(ctx)
		}
	}
}

func (s *Syncer) sync(ctx context.Context) {
	// Advisory lock to ensure single runner in multi-replica
	var locked bool
	err := s.db.QueryRow(ctx, `SELECT pg_try_advisory_lock(12345678)`).Scan(&locked)
	if err != nil || !locked {
		log.Debug().Msg("upstream sync skipped: could not acquire advisory lock")
		return
	}
	defer s.db.Exec(ctx, `SELECT pg_advisory_unlock(12345678)`)

	log.Info().Bool("startup", s.startup).Msg("syncing upstream decisions")

	resp, err := s.client.GetDecisions(ctx, s.startup)
	if err != nil {
		log.Error().Err(err).Msg("fetching upstream decisions")
		return
	}
	s.startup = false

	// Upsert new decisions
	for _, d := range resp.New {
		duration, _ := time.ParseDuration(d.Duration)
		if duration == 0 {
			duration = 24 * time.Hour
		}
		decision := &models.Decision{
			UUID:      d.UUID,
			Origin:    "upstream-capi",
			Type:      d.Type,
			Scope:     d.Scope,
			Value:     d.Value,
			Duration:  duration,
			Scenario:  &d.Scenario,
			ExpiresAt: time.Now().Add(duration),
		}
		if err := queries.UpsertUpstreamDecision(ctx, s.db, decision); err != nil {
			log.Error().Err(err).Str("uuid", d.UUID).Msg("upserting upstream decision")
		}
	}

	// Mark deleted
	for _, d := range resp.Deleted {
		if err := queries.MarkUpstreamDecisionDeleted(ctx, s.db, d.UUID); err != nil {
			log.Error().Err(err).Str("uuid", d.UUID).Msg("deleting upstream decision")
		}
	}

	// Update sync state
	_, _ = s.db.Exec(ctx, `
		UPDATE upstream_sync_state
		SET last_sync_at = NOW(), decision_count = (
			SELECT COUNT(*) FROM decisions WHERE origin = 'upstream-capi' AND is_deleted = FALSE
		)
		WHERE id = 1
	`)

	log.Info().
		Int("new", len(resp.New)).
		Int("deleted", len(resp.Deleted)).
		Msg("upstream sync complete")
}
