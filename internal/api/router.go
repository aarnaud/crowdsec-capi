package api

import (
	"crypto/subtle"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/aarnaud/crowdsec-central-api/internal/api/admin"
	"github.com/aarnaud/crowdsec-central-api/internal/api/authapi"
	v2 "github.com/aarnaud/crowdsec-central-api/internal/api/v2"
	"github.com/aarnaud/crowdsec-central-api/internal/auth"
	"github.com/aarnaud/crowdsec-central-api/internal/config"
	"github.com/aarnaud/crowdsec-central-api/internal/service"
	"github.com/aarnaud/crowdsec-central-api/internal/web"
)

// ipRateLimiter is a simple per-IP sliding-window rate limiter.
type ipRateLimiter struct {
	mu      sync.Mutex
	entries map[string][]time.Time
	limit   int
	window  time.Duration
}

func newIPRateLimiter(limit int, window time.Duration) *ipRateLimiter {
	return &ipRateLimiter{entries: make(map[string][]time.Time), limit: limit, window: window}
}

func (rl *ipRateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-rl.window)
	times := rl.entries[ip]
	var valid []time.Time
	for _, t := range times {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	if len(valid) >= rl.limit {
		rl.entries[ip] = valid
		return false
	}
	rl.entries[ip] = append(valid, now)
	return true
}

func rateLimitMiddleware(rl *ipRateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr
			if idx := strings.LastIndex(ip, ":"); idx > 0 {
				ip = ip[:idx]
			}
			if !rl.allow(ip) {
				w.Header().Set("Content-Type", "application/json")
				http.Error(w, `{"message":"too many requests"}`, http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

func NewRouter(
	pool *pgxpool.Pool,
	cfg *config.Config,
	jwtMgr *auth.JWTManager,
	signalSvc *service.SignalService,
	authHandlers *authapi.Handlers,
	sessionMgr *auth.SessionManager,
) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(chiZerologLogger)
	r.Use(middleware.Recoverer)
	r.Use(securityHeadersMiddleware)
	// Limit request bodies to 4 MB globally (signals handler is the largest consumer)
	r.Use(middleware.RequestSize(4 << 20))

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

	// Auth (public)
	r.Get("/auth/config", authHandlers.ConfigHandler())
	r.Get("/auth/session", authHandlers.SessionHandler())
	r.Get("/auth/login", authHandlers.LoginHandler())
	r.Get("/auth/callback", authHandlers.CallbackHandler())
	r.Get("/auth/logout", authHandlers.LogoutHandler())

	// v2 public endpoints — rate-limited: 10 requests per minute per IP
	authLimiter := newIPRateLimiter(10, time.Minute)
	r.With(rateLimitMiddleware(authLimiter)).Post("/v2/watchers", v2.RegisterHandler(pool, jwtMgr))
	r.With(rateLimitMiddleware(authLimiter)).Post("/v2/watchers/login", v2.LoginHandler(pool, jwtMgr))

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
		r.Use(adminAuthMiddleware(cfg, sessionMgr))
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

func adminAuthMiddleware(cfg *config.Config, sessionMgr *auth.SessionManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 1. Session cookie (set after OIDC login)
			if sessionMgr != nil {
				if cookie, err := r.Cookie(auth.SessionCookieName); err == nil {
					if _, err := sessionMgr.Verify(cookie.Value); err == nil {
						next.ServeHTTP(w, r)
						return
					}
				}
			}

			// 2. Bearer API key (constant-time compare to prevent timing attacks)
			if cfg.Admin.APIKey != "" {
				got := r.Header.Get("Authorization")
				want := "Bearer " + cfg.Admin.APIKey
				if subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1 {
					next.ServeHTTP(w, r)
					return
				}
			}

			// 3. HTTP Basic Auth (constant-time compare to prevent timing attacks)
			user, pass, ok := r.BasicAuth()
			if ok {
				userOK := subtle.ConstantTimeCompare([]byte(user), []byte(cfg.Admin.Username)) == 1
				passOK := subtle.ConstantTimeCompare([]byte(pass), []byte(cfg.Admin.Password)) == 1
				if userOK && passOK {
					next.ServeHTTP(w, r)
					return
				}
			}

			// Only send WWW-Authenticate for browser navigation requests.
			// Omitting it prevents the browser's native Basic Auth dialog
			// from appearing when the UI or scripts make JSON API calls.
			if !isJSONRequest(r) {
				w.Header().Set("WWW-Authenticate", `Basic realm="admin"`)
			}
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		})
	}
}

// isJSONRequest returns true for fetch/XHR calls that should not trigger the
// browser's native Basic Auth dialog. Detected via Accept or X-Requested-With.
func isJSONRequest(r *http.Request) bool {
	return r.Header.Get("X-Requested-With") == "XMLHttpRequest" ||
		strings.Contains(r.Header.Get("Accept"), "application/json")
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
