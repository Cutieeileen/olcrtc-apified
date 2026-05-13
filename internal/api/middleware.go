// Package api provides the HTTP REST API for channel management.
package api

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// authMiddleware returns a handler that checks the Authorization header
// against the master key using constant-time comparison.
func authMiddleware(masterKey string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			writeError(w, http.StatusUnauthorized, "missing authorization header")
			return
		}

		const bearerPrefix = "Bearer "
		if !strings.HasPrefix(auth, bearerPrefix) {
			writeError(w, http.StatusUnauthorized, "invalid authorization format")
			return
		}

		token := auth[len(bearerPrefix):]
		if subtle.ConstantTimeCompare([]byte(token), []byte(masterKey)) != 1 {
			writeError(w, http.StatusUnauthorized, "invalid master key")
			return
		}

		next.ServeHTTP(w, r)
	})
}
