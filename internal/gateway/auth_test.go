// SPDX-License-Identifier: GPL-3.0-only

package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGenerateTokenIsRandomAndNonEmpty(t *testing.T) {
	a, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	b, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	if a == "" || b == "" {
		t.Fatalf("expected non-empty tokens")
	}
	if a == b {
		t.Fatalf("expected two calls to produce different tokens")
	}
}

func TestRequireBearerRejectsMissingAndWrongToken(t *testing.T) {
	mw := RequireBearer("correct-token")
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	cases := []struct {
		name   string
		header string
		want   int
	}{
		{"missing header", "", http.StatusUnauthorized},
		{"wrong scheme", "Basic correct-token", http.StatusUnauthorized},
		{"wrong token", "Bearer wrong-token", http.StatusUnauthorized},
		{"correct token", "Bearer correct-token", http.StatusOK},
	}

	for _, c := range cases {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if c.header != "" {
			req.Header.Set("Authorization", c.header)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != c.want {
			t.Errorf("%s: got status %d, want %d", c.name, rec.Code, c.want)
		}
	}
}
