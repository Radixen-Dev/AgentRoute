// SPDX-License-Identifier: GPL-3.0-only

package gateway

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
)

// GenerateToken returns a fresh, cryptographically random session token
// used as the Bearer credential the gateway requires on every request.
// A new token is generated each time `agentroute up` starts the gateway;
// it is written into the linked platform's config (e.g. Claude Code's
// ANTHROPIC_AUTH_TOKEN) and never reused across restarts.
func GenerateToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("gateway: generate token: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// RequireBearer returns middleware that rejects any request whose
// "Authorization: Bearer <token>" header does not match expectedToken
// using a constant-time comparison.
func RequireBearer(expectedToken string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			const prefix = "Bearer "
			if !strings.HasPrefix(header, prefix) {
				http.Error(w, "missing bearer token", http.StatusUnauthorized)
				return
			}
			got := strings.TrimPrefix(header, prefix)
			if subtle.ConstantTimeCompare([]byte(got), []byte(expectedToken)) != 1 {
				http.Error(w, "invalid bearer token", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
