package auth

import (
	"context"
	"net/http"
	"strings"
)

type contextKey string

const MachineIDKey contextKey = "machine_id"

func JWTMiddleware(jwtMgr *JWTManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") {
				http.Error(w, `{"message":"missing or invalid authorization header"}`, http.StatusUnauthorized)
				return
			}
			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			claims, err := jwtMgr.Verify(tokenStr)
			if err != nil {
				http.Error(w, `{"message":"invalid or expired token"}`, http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), MachineIDKey, claims.MachineID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetMachineID(ctx context.Context) string {
	v, _ := ctx.Value(MachineIDKey).(string)
	return v
}
