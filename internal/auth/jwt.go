package auth

import (
	"fmt"
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
