package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/aarnaud/crowdsec-central-api/internal/api/admin"
	v2 "github.com/aarnaud/crowdsec-central-api/internal/api/v2"
	"github.com/aarnaud/crowdsec-central-api/internal/auth"
	"github.com/aarnaud/crowdsec-central-api/internal/config"
	"github.com/aarnaud/crowdsec-central-api/internal/service"
	"github.com/aarnaud/crowdsec-central-api/internal/web"
)

func NewRouter(pool *pgxpool.Pool, cfg *config.Config, jwtMgr *auth.JWTManager, signalSvc *service.SignalService) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(chiZerologLogger)
	r.Use(middleware.Recoverer)

	// Static dashboard
	staticFS := http.FS(web.Static)
	r.Handle("/ui/*", http.StripPrefix("/ui", http.FileServer(staticFS)))
	r.Get("/ui", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/ui/static/index.html", http.StatusFound)
	})

	// Health
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			http.Error(w, "db not ready", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// v2 public endpoints
	r.Post("/v2/watchers", v2.RegisterHandler(pool, jwtMgr))
	r.Post("/v2/watchers/login", v2.LoginHandler(pool, jwtMgr))

	// v2 authenticated endpoints
	r.Group(func(r chi.Router) {
		r.Use(auth.JWTMiddleware(jwtMgr))
		r.Post("/v2/watchers/enroll", v2.EnrollHandler(pool))
		r.Post("/v2/watchers/reset", v2.ResetPasswordHandler(pool))
		r.Delete("/v2/watchers/self", v2.DeleteSelfHandler(pool))
		r.Post("/v2/signals", v2.SignalsHandler(signalSvc))
		r.Get("/v2/decisions/stream", v2.DecisionStreamHandler(pool))
		r.Post("/v2/decisions/sync", v2.DecisionSyncHandler(pool))
		r.Post("/v2/metrics", v2.MetricsHandler(pool))
		r.Post("/v2/usage-metrics", v2.MetricsHandler(pool))
		r.Get("/v2/heartbeat", v2.HeartbeatHandler(pool))
		r.Get("/v2/allowlists", v2.AllowlistsGetHandler(pool))
		r.Head("/v2/allowlists", v2.AllowlistsHeadHandler(pool))
		r.Post("/v2/allowlists/{name}", v2.AllowlistsPostHandler(pool))
		r.Get("/v2/papi/v1/decisions", v2.PAPIHandler())
	})

	// Admin endpoints
	r.Group(func(r chi.Router) {
		r.Use(adminAuthMiddleware(cfg))
		r.Get("/admin/machines", admin.ListMachinesHandler(pool))
		r.Put("/admin/machines/{machine_id}/block", admin.BlockMachineHandler(pool))
		r.Put("/admin/machines/{machine_id}/unblock", admin.UnblockMachineHandler(pool))
		r.Delete("/admin/machines/{machine_id}", admin.DeleteMachineHandler(pool))
		r.Get("/admin/decisions", admin.ListDecisionsHandler(pool))
		r.Post("/admin/decisions", admin.CreateDecisionHandler(pool, cfg.Decisions.DefaultDuration))
		r.Delete("/admin/decisions/{uuid}", admin.DeleteDecisionHandler(pool))
		r.Get("/admin/allowlists", admin.ListAllowlistsHandler(pool))
		r.Post("/admin/allowlists", admin.CreateAllowlistHandler(pool))
		r.Delete("/admin/allowlists/{id}", admin.DeleteAllowlistHandler(pool))
		r.Post("/admin/allowlists/{id}/entries", admin.AddAllowlistEntryHandler(pool))
		r.Get("/admin/enrollment-keys", admin.ListEnrollmentKeysHandler(pool))
		r.Post("/admin/enrollment-keys", admin.CreateEnrollmentKeyHandler(pool))
		r.Delete("/admin/enrollment-keys/{id}", admin.DeleteEnrollmentKeyHandler(pool))
		r.Get("/admin/upstream", admin.UpstreamStatusHandler(pool))
		r.Get("/admin/stats", admin.StatsHandler(pool))
	})

	return r
}

func adminAuthMiddleware(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Try Bearer API key first
			if cfg.Admin.APIKey != "" {
				bearer := r.Header.Get("Authorization")
				if bearer == "Bearer "+cfg.Admin.APIKey {
					next.ServeHTTP(w, r)
					return
				}
			}
			// Fall back to Basic Auth
			user, pass, ok := r.BasicAuth()
			if !ok || user != cfg.Admin.Username || pass != cfg.Admin.Password {
				w.Header().Set("WWW-Authenticate", `Basic realm="admin"`)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func chiZerologLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Info().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("remote_addr", r.RemoteAddr).
			Msg("request")
		next.ServeHTTP(w, r)
	})
}
