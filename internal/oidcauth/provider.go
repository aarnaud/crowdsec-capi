package oidcauth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/aarnaud/crowdsec-central-api/internal/config"
)

type Provider struct {
	oauth2Config oauth2.Config
	verifier     *gooidc.IDTokenVerifier
	cfg          config.OIDCConfig
}

func New(ctx context.Context, cfg config.OIDCConfig) (*Provider, error) {
	provider, err := gooidc.NewProvider(ctx, cfg.Issuer)
	if err != nil {
		return nil, fmt.Errorf("OIDC discovery for %q: %w", cfg.Issuer, err)
	}

	scopes := cfg.Scopes
	if len(scopes) == 0 {
		scopes = []string{gooidc.ScopeOpenID, "profile", "email"}
	}

	return &Provider{
		oauth2Config: oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Endpoint:     provider.Endpoint(),
			Scopes:       scopes,
		},
		verifier: provider.Verifier(&gooidc.Config{ClientID: cfg.ClientID}),
		cfg:      cfg,
	}, nil
}

func (p *Provider) AuthCodeURL(state string) string {
	return p.oauth2Config.AuthCodeURL(state)
}

func (p *Provider) Exchange(ctx context.Context, code string) (email, name string, err error) {
	token, err := p.oauth2Config.Exchange(ctx, code)
	if err != nil {
		return "", "", fmt.Errorf("token exchange: %w", err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return "", "", fmt.Errorf("no id_token in response")
	}

	idToken, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return "", "", fmt.Errorf("verifying id_token: %w", err)
	}

	var claims struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return "", "", fmt.Errorf("extracting claims: %w", err)
	}

	if len(p.cfg.AllowedEmails) > 0 {
		allowed := false
		for _, e := range p.cfg.AllowedEmails {
			if e == claims.Email {
				allowed = true
				break
			}
		}
		if !allowed {
			return "", "", fmt.Errorf("email %q not in allowed list", claims.Email)
		}
	}

	if len(p.cfg.AllowedDomains) > 0 {
		domain := emailDomain(claims.Email)
		allowed := false
		for _, d := range p.cfg.AllowedDomains {
			if d == domain {
				allowed = true
				break
			}
		}
		if !allowed {
			return "", "", fmt.Errorf("email domain %q not allowed", domain)
		}
	}

	return claims.Email, claims.Name, nil
}

func RandomState() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func emailDomain(email string) string {
	if i := strings.LastIndex(email, "@"); i >= 0 {
		return email[i+1:]
	}
	return ""
}
