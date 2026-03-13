package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type MachineClaims struct {
	MachineID string `json:"machine_id"`
	jwt.RegisteredClaims
}

type JWTManager struct {
	secret []byte
	ttl    time.Duration
}

func NewJWTManager(secret []byte, ttl time.Duration) *JWTManager {
	return &JWTManager{secret: secret, ttl: ttl}
}

func (m *JWTManager) Sign(machineID string) (string, time.Time, error) {
	exp := time.Now().Add(m.ttl)
	claims := MachineClaims{
		MachineID: machineID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(exp),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "crowdsec-local-capi",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("signing token: %w", err)
	}
	return signed, exp, nil
}

// SignURL returns a short-lived HMAC-signed token for unauthenticated allowlist downloads.
// The token encodes the allowlist name and expiry so it cannot be reused or forged.
func (m *JWTManager) SignURL(name string, ttl time.Duration) (token string, exp int64) {
	exp = time.Now().Add(ttl).Unix()
	msg := name + ":" + strconv.FormatInt(exp, 10)
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(msg))
	return hex.EncodeToString(mac.Sum(nil)), exp
}

// ValidateSignedURL checks an allowlist download token.
func (m *JWTManager) ValidateSignedURL(name, token string, exp int64) bool {
	if time.Now().Unix() > exp {
		return false
	}
	msg := name + ":" + strconv.FormatInt(exp, 10)
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(msg))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(token), []byte(expected))
}

func (m *JWTManager) Verify(tokenStr string) (*MachineClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &MachineClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("parsing token: %w", err)
	}
	claims, ok := token.Claims.(*MachineClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}
	return claims, nil
}
