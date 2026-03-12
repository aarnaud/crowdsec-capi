package upstream

import (
	"context"
	"crypto/sha1"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/aarnaud/crowdsec-central-api/internal/db/queries"
	"github.com/aarnaud/crowdsec-central-api/internal/models"
)

// upstreamDecisionUUID derives a stable UUID v5-like identifier from scope+value
// so that upstream decisions (which carry no UUID in the API response) can be
// reliably upserted and deleted without changing the DB schema.
func upstreamDecisionUUID(scope, value string) string {
	h := sha1.Sum([]byte("upstream-capi\x00" + scope + "\x00" + value))
	// Overlay UUID v5 version and variant bits onto the first 16 bytes.
	h[6] = (h[6] & 0x0f) | 0x50
	h[8] = (h[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		h[0:4], h[4:6], h[6:8], h[8:10], h[10:16])
}

// minSyncInterval is the minimum time between upstream API calls.
// The official CrowdSec CAPI enforces a 2-hour window per machine.
const minSyncInterval = 2 * time.Hour

// startupCursorTTL is how long the upstream CAPI is expected to retain a
// machine's delta cursor. After this window we fall back to startup=true to
// get a full snapshot instead of a potentially incomplete delta.
const startupCursorTTL = 24 * time.Hour

type Syncer struct {
	client        *Client
	db            *pgxpool.Pool
	interval      time.Duration
	enrollmentKey string
}

func NewSyncer(client *Client, db *pgxpool.Pool, interval time.Duration, enrollmentKey string) *Syncer {
	if interval < minSyncInterval {
		log.Warn().
			Dur("configured", interval).
			Dur("minimum", minSyncInterval).
			Msg("upstream sync_interval is below the 2h minimum enforced by the official CAPI — the syncer will self-limit")
	}
	return &Syncer{
		client:        client,
		db:            db,
		interval:      interval,
		enrollmentKey: enrollmentKey,
	}
}

func (s *Syncer) Run(ctx context.Context) {
	log.Info().Msg("upstream syncer started")

	// Restore token from DB to avoid an unnecessary re-login on every restart.
	s.loadTokenFromDB(ctx)

	// Register and enroll once (idempotent — safe to call on every startup).
	s.setup(ctx)

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

func (s *Syncer) loadTokenFromDB(ctx context.Context) {
	var token *string
	var tokenExp *time.Time
	_ = s.db.QueryRow(ctx, `
		SELECT token, token_expires_at FROM upstream_sync_state WHERE id = 1
	`).Scan(&token, &tokenExp)
	if token != nil && *token != "" && tokenExp != nil && time.Now().Before(*tokenExp) {
		s.client.SetToken(*token, *tokenExp)
		log.Debug().Msg("upstream: restored token from DB")
	}
}

// setup registers the machine with the upstream CAPI (idempotent) and, if an
// enrollment key is configured, enrolls it with the CrowdSec console. Both
// operations are only performed when necessary (enrollment is skipped once the
// enrolled_at timestamp is persisted in the DB).
func (s *Syncer) setup(ctx context.Context) {
	// Ensure the singleton row exists (defensive: migration seed may have been lost).
	_, _ = s.db.Exec(ctx, `INSERT INTO upstream_sync_state (id) VALUES (1) ON CONFLICT DO NOTHING`)

	// Persist the machine_id so the admin UI can display it.
	_, _ = s.db.Exec(ctx, `UPDATE upstream_sync_state SET machine_id = $1 WHERE id = 1`, s.client.MachineID())

	// Step 1: register the machine. A 409 Conflict (already registered) is OK.
	if err := s.client.Register(ctx); err != nil {
		log.Error().Err(err).Msg("upstream: machine registration failed")
		return
	}
	log.Debug().Msg("upstream: machine registered (or already exists)")

	// Step 2: login to obtain a token (needed for enrollment and syncing).
	if err := s.client.Login(ctx); err != nil {
		log.Error().Err(err).Msg("upstream: login failed during setup")
		return
	}

	// Step 3: enroll with the console if a key is configured and not yet done.
	if s.enrollmentKey == "" {
		return
	}

	var enrolledAt *time.Time
	_ = s.db.QueryRow(ctx, `SELECT enrolled_at FROM upstream_sync_state WHERE id = 1`).Scan(&enrolledAt)
	if enrolledAt != nil {
		log.Debug().Time("enrolled_at", *enrolledAt).Msg("upstream: already enrolled, skipping")
		return
	}

	if err := s.client.Enroll(ctx, s.enrollmentKey); err != nil {
		log.Error().Err(err).Msg("upstream: enrollment failed")
		return
	}

	log.Info().Msg("upstream: enrollment request sent — approve the machine at app.crowdsec.net to activate blocklist sync")

	// Block until the console admin approves the enrollment.
	// The sync loop and its ticker do not start until this returns.
	if !s.waitForEnrollmentApproval(ctx) {
		return // context canceled (SIGTERM)
	}

	_, _ = s.db.Exec(ctx, `UPDATE upstream_sync_state SET enrolled_at = NOW() WHERE id = 1`)
	log.Info().Msg("upstream: enrollment approved — blocklist sync starting")
}

// waitForEnrollmentApproval polls the upstream decision stream every 30 s until
// the machine is approved in the CrowdSec console (indicated by a successful
// 200 response). Returns false only when ctx is canceled.
func (s *Syncer) waitForEnrollmentApproval(ctx context.Context) bool {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
			_, err := s.client.GetDecisions(ctx, false)
			if err == nil {
				return true
			}
			log.Info().Msg("upstream: still waiting for enrollment approval at app.crowdsec.net")
		}
	}
}

func (s *Syncer) sync(ctx context.Context) {
	// Acquire a dedicated connection so that the session-level advisory lock
	// and its corresponding unlock execute on the same PostgreSQL backend.
	// With pgxpool, successive Exec/QueryRow calls may use different backend
	// connections, which would cause pg_advisory_unlock to silently no-op and
	// leak the lock until the backend is recycled.
	conn, err := s.db.Acquire(ctx)
	if err != nil {
		log.Error().Err(err).Msg("upstream sync: failed to acquire DB connection")
		return
	}
	defer conn.Release() // runs after pg_advisory_unlock (LIFO defer order)

	var locked bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock(12345678)`).Scan(&locked); err != nil || !locked {
		log.Debug().Msg("upstream sync skipped: could not acquire advisory lock")
		return
	}
	// Explicit unlock on the same connection before Release returns it to the pool.
	// Use context.Background() so a shutdown signal doesn't prevent the unlock.
	defer conn.Exec(context.Background(), `SELECT pg_advisory_unlock(12345678)`)

	// Enforce the 2h rate limit. This check runs inside the advisory lock so
	// replicas that start simultaneously both see the same last_sync_at.
	var lastSyncAt *time.Time
	_ = s.db.QueryRow(ctx, `SELECT last_sync_at FROM upstream_sync_state WHERE id = 1`).Scan(&lastSyncAt)
	if lastSyncAt != nil {
		elapsed := time.Since(*lastSyncAt)
		if elapsed < minSyncInterval {
			remaining := (minSyncInterval - elapsed).Truncate(time.Second)
			log.Debug().
				Dur("remaining", remaining).
				Msg("upstream sync skipped: 2h minimum interval not yet elapsed")
			return
		}
	}

	// Determine startup vs delta from persisted last_startup_at.
	// startup=true (full snapshot) when we have never synced or when the upstream
	// may have discarded our delta cursor (older than startupCursorTTL).
	var lastStartupAt *time.Time
	_ = s.db.QueryRow(ctx, `SELECT last_startup_at FROM upstream_sync_state WHERE id = 1`).Scan(&lastStartupAt)
	startup := lastStartupAt == nil || time.Since(*lastStartupAt) > startupCursorTTL

	log.Info().Bool("startup", startup).Msg("syncing upstream decisions")

	// Detach from the application shutdown context so that a SIGTERM arriving
	// mid-sync does not cancel DB writes and leave decisions in a partial state.
	// A separate timeout caps the total sync duration.
	syncCtx, syncCancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer syncCancel()

	resp, err := s.client.GetDecisions(syncCtx, startup)
	if err != nil {
		log.Error().Err(err).Msg("fetching upstream decisions")
		return
	}

	// Upsert new decisions into local cache so they are served to downstream agents.
	// The real API returns a flat array (despite the swagger showing grouped objects).
	newCount := 0
	for _, d := range resp.New {
		duration, _ := time.ParseDuration(d.Duration)
		if duration == 0 {
			duration = 24 * time.Hour
		}
		decision := &models.Decision{
			UUID:      upstreamDecisionUUID(d.Scope, d.Value),
			Origin:    "upstream-capi",
			Type:      d.Type,
			Scope:     d.Scope,
			Value:     d.Value,
			Duration:  duration,
			Scenario:  &d.Scenario,
			ExpiresAt: time.Now().Add(duration),
		}
		if err := queries.UpsertUpstreamDecision(syncCtx, s.db, decision); err != nil {
			log.Error().Err(err).Str("scope", d.Scope).Str("value", d.Value).Msg("upserting upstream decision")
		} else {
			newCount++
		}
	}

	// Soft-delete removed decisions.
	deletedCount := 0
	for _, d := range resp.Deleted {
		uuid := upstreamDecisionUUID(d.Scope, d.Value)
		if err := queries.MarkUpstreamDecisionDeleted(syncCtx, s.db, uuid); err != nil {
			log.Error().Err(err).Str("scope", d.Scope).Str("value", d.Value).Msg("deleting upstream decision")
		} else {
			deletedCount++
		}
	}

	// Persist sync state and the refreshed JWT so the next restart skips re-login.
	now := time.Now()
	token, tokenExp := s.client.GetToken()

	if startup {
		_, _ = s.db.Exec(syncCtx, `
			UPDATE upstream_sync_state
			SET last_sync_at     = $1,
			    last_startup_at  = $1,
			    token            = $2,
			    token_expires_at = $3,
			    decision_count   = (
			        SELECT COUNT(*) FROM decisions
			        WHERE origin = 'upstream-capi' AND is_deleted = FALSE
			    )
			WHERE id = 1
		`, now, token, tokenExp)
	} else {
		_, _ = s.db.Exec(syncCtx, `
			UPDATE upstream_sync_state
			SET last_sync_at     = $1,
			    token            = $2,
			    token_expires_at = $3,
			    decision_count   = (
			        SELECT COUNT(*) FROM decisions
			        WHERE origin = 'upstream-capi' AND is_deleted = FALSE
			    )
			WHERE id = 1
		`, now, token, tokenExp)
	}

	log.Info().
		Int("new", newCount).
		Int("deleted", deletedCount).
		Bool("startup", startup).
		Msg("upstream sync complete")
}
