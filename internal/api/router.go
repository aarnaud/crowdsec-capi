package api

import (
	"compress/gzip"
	"crypto/subtle"
	"io"
	"io/fs"
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

// gunzipMiddleware transparently decompresses gzip-encoded request bodies.
// The v3 CrowdSec agent compresses request bodies with gzip.
func gunzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Encoding") == "gzip" {
			gr, err := gzip.NewReader(r.Body)
			if err != nil {
				http.Error(w, `{"message":"invalid gzip body"}`, http.StatusBadRequest)
				return
			}
			defer gr.Close()
			r.Body = io.NopCloser(gr)
			r.Header.Del("Content-Encoding")
		}
		next.ServeHTTP(w, r)
	})
}

// apiStripSlashes removes trailing slashes on /v2/ and /v3/ API paths only,
// leaving UI and other routes unaffected.
func apiStripSlashes(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(r.URL.Path) > 4 &&
			r.URL.Path[len(r.URL.Path)-1] == '/' &&
			(strings.HasPrefix(r.URL.Path, "/v2/") || strings.HasPrefix(r.URL.Path, "/v3/")) {
			r.URL.Path = r.URL.Path[:len(r.URL.Path)-1]
		}
		next.ServeHTTP(w, r)
	})
}

func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'")
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
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		log.Warn().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("query", r.URL.RawQuery).
			Str("remote_addr", r.RemoteAddr).
			Msg("404 no route matched")
		http.Error(w, "404 page not found", http.StatusNotFound)
	})
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(apiStripSlashes)
	r.Use(chiZerologLogger)
	r.Use(middleware.Recoverer)
	r.Use(securityHeadersMiddleware)
	r.Use(gunzipMiddleware)
	// Limit request bodies to 4 MB globally (signals handler is the largest consumer)
	r.Use(middleware.RequestSize(4 << 20))

	// Static dashboard
	subFS, err := fs.Sub(web.Static, "static")
	if err != nil {
		log.Fatal().Err(err)
	}

	r.Handle("/ui/*", http.StripPrefix("/ui/", http.FileServer(http.FS(subFS))))
	r.Handle("/ui/", http.StripPrefix("/ui/", http.FileServer(http.FS(subFS))))
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/ui/", http.StatusFound)
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

	// v2/v3 public endpoints — rate-limited: 10 requests per minute per IP
	authLimiter := newIPRateLimiter(10, time.Minute)
	//r.With(rateLimitMiddleware(authLimiter)).Post("/v2/watchers", v2.RegisterHandler(pool, jwtMgr))
	//r.With(rateLimitMiddleware(authLimiter)).Post("/v2/watchers/login", v2.LoginHandler(pool, jwtMgr))
	r.With(rateLimitMiddleware(authLimiter)).Post("/v3/watchers", v2.RegisterHandler(pool, jwtMgr))
	r.With(rateLimitMiddleware(authLimiter)).Post("/v3/watchers/login", v2.V3LoginHandler(pool, jwtMgr))

	// Per-machine-ID rate limiter for signal batches: 60 per minute
	signalLimiter := newIPRateLimiter(60, time.Minute)
	signalRateLimitMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			machineID := auth.GetMachineID(r.Context())
			if !signalLimiter.allow(machineID) {
				w.Header().Set("Content-Type", "application/json")
				http.Error(w, `{"message":"too many requests"}`, http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}

	// authenticated endpoints
	r.Group(func(r chi.Router) {
		r.Use(auth.JWTMiddleware(jwtMgr))

		// shared routes (same handler for v2 and v3)
		for _, p := range []string{"/v2", "/v3"} {
			r.Post(p+"/watchers/enroll", v2.EnrollHandler(pool))
			r.Post(p+"/watchers/reset", v2.ResetPasswordHandler(pool))
			r.Delete(p+"/watchers/self", v2.DeleteSelfHandler(pool))
			r.With(signalRateLimitMiddleware).Post(p+"/signals", v2.SignalsHandler(signalSvc))
			r.Post(p+"/decisions/sync", v2.DecisionSyncHandler(pool))
			r.Post(p+"/metrics", v2.MetricsHandler(pool))
			r.Post(p+"/usage-metrics", v2.MetricsHandler(pool))
			r.Get(p+"/heartbeat", v2.HeartbeatHandler(pool))
			r.Get(p+"/allowlists", v2.AllowlistsGetHandler(pool))
			r.Head(p+"/allowlists", v2.AllowlistsHeadHandler(pool))
			r.Get(p+"/allowlists/check/{value}", v2.AllowlistCheckGetHandler(pool))
			r.Head(p+"/allowlists/check/{value}", v2.AllowlistCheckHeadHandler(pool))
			r.Post(p+"/allowlists/check", v2.AllowlistCheckBulkHandler(pool))
			r.Get(p+"/allowlists/{name}", v2.AllowlistGetByNameHandler(pool))
			r.Post(p+"/allowlists/{name}", v2.AllowlistsPostHandler(pool))
			r.Get(p+"/papi/v1/decisions", v2.PAPIHandler())
		}

		// version-specific decision stream handlers
		r.Get("/v2/decisions/stream", v2.DecisionStreamHandler(pool))
		r.Get("/v3/decisions/stream", v2.V3DecisionStreamHandler(pool))

		// PAPI (Push API) — same JWT auth, uses /v1/ prefix
		r.Get("/v1/permissions", v2.PAPIPermissionsHandler())
		r.Get("/v1/decisions/stream/poll", v2.PAPIPollHandler())
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
		r.Get("/admin/allowlists/{id}/entries", admin.ListAllowlistEntriesHandler(pool))
		r.Post("/admin/allowlists/{id}/entries", admin.AddAllowlistEntryHandler(pool))
		r.Put("/admin/allowlists/{id}/entries/{entry_id}", admin.UpdateAllowlistEntryHandler(pool))
		r.Delete("/admin/allowlists/{id}/entries/{entry_id}", admin.DeleteAllowlistEntryHandler(pool))
		r.Get("/admin/enrollment-keys", admin.ListEnrollmentKeysHandler(pool))
		r.Post("/admin/enrollment-keys", admin.CreateEnrollmentKeyHandler(pool))
		r.Delete("/admin/enrollment-keys/{id}", admin.DeleteEnrollmentKeyHandler(pool))
		r.Get("/admin/signals", admin.ListSignalsHandler(pool))
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
