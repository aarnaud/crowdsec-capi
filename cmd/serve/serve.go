package serve

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/aarnaud/crowdsec-central-api/internal/allowlists"
	"github.com/aarnaud/crowdsec-central-api/internal/api"
	"github.com/aarnaud/crowdsec-central-api/internal/api/authapi"
	"github.com/aarnaud/crowdsec-central-api/internal/auth"
	"github.com/aarnaud/crowdsec-central-api/internal/config"
	"github.com/aarnaud/crowdsec-central-api/internal/db"
	"github.com/aarnaud/crowdsec-central-api/internal/db/queries"
	"github.com/aarnaud/crowdsec-central-api/internal/oidcauth"
	"github.com/aarnaud/crowdsec-central-api/internal/service"
	"github.com/aarnaud/crowdsec-central-api/internal/upstream"
)

var cfgFile string

func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the CrowdSec Central API server",
		RunE:  runServe,
	}
	cmd.Flags().StringVarP(&cfgFile, "config", "c", "", "config file path")
	return cmd
}

func runServe(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return err
	}

	// Setup logging
	if cfg.Log.Format == "pretty" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}
	level, err := zerolog.ParseLevel(cfg.Log.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run migrations
	log.Info().Msg("running database migrations")
	if err := db.RunMigrations(cfg.Database.DSN); err != nil {
		return err
	}

	// Connect to DB
	pool, err := db.NewPool(ctx, cfg.Database.DSN)
	if err != nil {
		return err
	}
	defer pool.Close()

	// Init OIDC provider if configured
	var oidcProvider *oidcauth.Provider
	if cfg.Auth.OIDC.Enabled {
		log.Info().Str("issuer", cfg.Auth.OIDC.Issuer).Msg("initializing OIDC provider")
		oidcProvider, err = oidcauth.New(ctx, cfg.Auth.OIDC)
		if err != nil {
			return fmt.Errorf("OIDC init: %w", err)
		}
		log.Info().Msg("OIDC provider ready")
		if len(cfg.Auth.OIDC.AllowedEmails) == 0 && len(cfg.Auth.OIDC.AllowedDomains) == 0 {
			log.Warn().Msg("OIDC: no allowed_emails or allowed_domains configured — any authenticated user can access the admin UI")
		}
	}

	// Load allowlists from file if configured
	if cfg.Allowlists.File != "" {
		log.Info().Str("file", cfg.Allowlists.File).Msg("loading allowlists from file")
		if err := allowlists.LoadFile(ctx, pool, cfg.Allowlists.File); err != nil {
			return fmt.Errorf("loading allowlists file: %w", err)
		}
	}

	// Generate admin password if not set and OIDC is not the sole auth method
	if cfg.Admin.Password == "" && !cfg.Auth.OIDC.Enabled {
		cfg.Admin.Password = generateSecret(16)
		log.Warn().
			Str("username", cfg.Admin.Username).
			Str("password", cfg.Admin.Password).
			Msg("no admin password configured — generated a random one (set CAPI_ADMIN_PASSWORD to persist)")
	}

	// Get or create a persistent JWT secret stored in the database
	jwtSecret, err := queries.GetOrCreateJWTSecret(ctx, pool)
	if err != nil {
		return fmt.Errorf("loading JWT secret: %w", err)
	}
	jwtMgr := auth.NewJWTManager(jwtSecret, cfg.Server.JWTTTL)
	sessionMgr := auth.NewSessionManager(jwtSecret, cfg.Server.JWTTTL)
	authHandlers := authapi.New(oidcProvider, sessionMgr, cfg.Server.JWTTTL, cfg.Server.SecureCookies)
	signalSvc := service.NewSignalService(pool, cfg.Decisions.DefaultDuration)

	// Start background goroutines
	go expiredDecisionCleaner(ctx, pool)

	// Start upstream syncer if enabled
	if cfg.Upstream.Enabled && cfg.Upstream.MachineID != "" {
		client := upstream.NewClient(cfg.Upstream.BaseURL, cfg.Upstream.MachineID, cfg.Upstream.Password)
		syncer := upstream.NewSyncer(client, pool, cfg.Upstream.SyncInterval)
		go syncer.Run(ctx)
	}

	router := api.NewRouter(pool, cfg, jwtMgr, signalSvc, authHandlers, sessionMgr)

	srv := &http.Server{
		Addr:         cfg.Server.Listen,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		log.Info().Str("addr", cfg.Server.Listen).Msg("server starting")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server error")
		}
	}()

	<-stop
	log.Info().Msg("shutting down")
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutCancel()
	cancel()
	return srv.Shutdown(shutCtx)
}

func generateSecret(nBytes int) string {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		panic("failed to generate secret: " + err.Error())
	}
	return hex.EncodeToString(b)
}

func expiredDecisionCleaner(ctx context.Context, pool *pgxpool.Pool) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, err := queries.SoftDeleteExpiredDecisions(ctx, pool)
			if err != nil {
				log.Error().Err(err).Msg("cleaning expired decisions")
			} else if n > 0 {
				log.Info().Int64("count", n).Msg("soft-deleted expired decisions")
			}
			n, err = queries.HardDeleteAgedDecisions(ctx, pool)
			if err != nil {
				log.Error().Err(err).Msg("hard-deleting aged decisions")
			} else if n > 0 {
				log.Info().Int64("count", n).Msg("hard-deleted aged soft-deleted decisions")
			}
		}
	}
}
