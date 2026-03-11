package authapi

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/aarnaud/crowdsec-central-api/internal/auth"
	"github.com/aarnaud/crowdsec-central-api/internal/oidcauth"
)

type Handlers struct {
	provider      *oidcauth.Provider
	sessions      *auth.SessionManager
	sessionTTL    time.Duration
	secureCookies bool
}

func New(provider *oidcauth.Provider, sessions *auth.SessionManager, sessionTTL time.Duration, secureCookies bool) *Handlers {
	return &Handlers{provider: provider, sessions: sessions, sessionTTL: sessionTTL, secureCookies: secureCookies}
}

// ConfigHandler returns public auth configuration so the UI knows which login mode to use.
func (h *Handlers) ConfigHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]bool{
			"oidc_enabled": h.provider != nil,
		})
	}
}

// SessionHandler returns the current session identity from the session cookie.
// Returns 200 with user info if authenticated, 401 without WWW-Authenticate if not.
// Never triggers the browser's native Basic Auth dialog.
func (h *Handlers) SessionHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(auth.SessionCookieName)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"authenticated":false}`))
			return
		}
		claims, err := h.sessions.Verify(cookie.Value)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"authenticated":false}`))
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"authenticated": true,
			"email":         claims.Email,
			"name":          claims.Name,
		})
	}
}

// LoginHandler initiates the OIDC auth code flow.
func (h *Handlers) LoginHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.provider == nil {
			http.Error(w, `{"message":"OIDC not configured"}`, http.StatusNotFound)
			return
		}
		state := oidcauth.RandomState()
		http.SetCookie(w, &http.Cookie{
			Name:     auth.StateCookieName,
			Value:    state,
			MaxAge:   600,
			HttpOnly: true,
			Secure:   h.secureCookies,
			SameSite: http.SameSiteLaxMode,
			Path:     "/",
		})
		http.Redirect(w, r, h.provider.AuthCodeURL(state), http.StatusFound)
	}
}

// CallbackHandler handles the OIDC redirect, validates the token, and sets a session cookie.
func (h *Handlers) CallbackHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.provider == nil {
			http.Error(w, `{"message":"OIDC not configured"}`, http.StatusNotFound)
			return
		}

		stateCookie, err := r.Cookie(auth.StateCookieName)
		if err != nil || stateCookie.Value != r.URL.Query().Get("state") {
			log.Warn().Msg("OIDC callback: invalid or missing state cookie, restarting flow")
			http.Redirect(w, r, "/auth/login", http.StatusFound)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: auth.StateCookieName, MaxAge: -1, Path: "/"})

		email, name, err := h.provider.Exchange(r.Context(), r.URL.Query().Get("code"))
		if err != nil {
			// Authorization codes are single-use and expire quickly. Redirect back to
			// login so the user can obtain a fresh code rather than seeing a raw error.
			log.Warn().Err(err).Msg("OIDC callback: token exchange failed, restarting flow")
			http.Redirect(w, r, "/auth/login", http.StatusFound)
			return
		}

		sessionToken, err := h.sessions.Create(email, name)
		if err != nil {
			log.Error().Err(err).Msg("OIDC callback: session creation failed")
			http.Error(w, "session creation failed", http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     auth.SessionCookieName,
			Value:    sessionToken,
			MaxAge:   int(h.sessionTTL / time.Second),
			HttpOnly: true,
			Secure:   h.secureCookies,
			SameSite: http.SameSiteLaxMode,
			Path:     "/",
		})
		http.Redirect(w, r, "/ui/", http.StatusFound)
	}
}

// LogoutHandler clears the session cookie.
func (h *Handlers) LogoutHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{
			Name:     auth.SessionCookieName,
			Value:    "",
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   h.secureCookies,
			SameSite: http.SameSiteLaxMode,
			Path:     "/",
		})
		http.Redirect(w, r, "/ui/", http.StatusFound)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
