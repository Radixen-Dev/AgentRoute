// SPDX-License-Identifier: GPL-3.0-only

package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// stubTranslator is a minimal Translator used only to verify that chi's
// route patterns in routeFor actually match every path AgentRoute needs
// to serve for that wire (e.g. Anthropic's /v1/messages AND
// /v1/messages/count_tokens), independent of any real translation logic.
type stubTranslator struct {
	wire Wire
	hits *[]string
}

func (s *stubTranslator) Wire() Wire { return s.wire }

func (s *stubTranslator) Handler(_ Upstream, _ ModelRouter, _ *RequestLog) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*s.hits = append(*s.hits, r.URL.Path)
		w.WriteHeader(http.StatusOK)
	})
}

func TestAnthropicRoutePatternMatchesMessagesAndCountTokens(t *testing.T) {
	var hits []string
	srv, err := New(Config{
		Port:        0,
		Token:       "tok",
		Upstream:    Upstream{},
		Router:      MapRouter{},
		Translators: []Translator{&stubTranslator{wire: WireAnthropic, hits: &hits}},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	go func() { _ = srv.Serve() }()
	t.Cleanup(func() { _ = srv.Shutdown(0) })

	base := "http://127.0.0.1:" + itoaTest(srv.Port())
	paths := []string{"/v1/messages", "/v1/messages/count_tokens"}
	for _, p := range paths {
		req, _ := http.NewRequest(http.MethodPost, base+p, nil)
		req.Header.Set("Authorization", "Bearer tok")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST %s: %v", p, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("POST %s: got status %d, want 200 (route did not match)", p, resp.StatusCode)
		}
	}

	if len(hits) != len(paths) {
		t.Fatalf("handler was hit %d times, want %d; hits=%v", len(hits), len(paths), hits)
	}
}

func itoaTest(n int) string {
	rec := httptest.NewRecorder()
	_ = rec
	// Avoid importing strconv twice across test files; simple manual itoa.
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
