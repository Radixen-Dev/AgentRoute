// SPDX-License-Identifier: GPL-3.0-only

package gateway

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func startTestServer(t *testing.T, upstreamURL, token string) *Server {
	t.Helper()
	srv, err := New(Config{
		Port:        0,
		Token:       token,
		Upstream:    Upstream{BaseURL: upstreamURL, APIKey: "fake-or-key"},
		Router:      MapRouter{"agentroute-balanced": "openrouter/some/model"},
		Translators: []Translator{&OpenAINativeTranslator{}},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	go func() {
		_ = srv.Serve()
	}()
	t.Cleanup(func() { _ = srv.Shutdown(2 * time.Second) })
	return srv
}

func TestOpenAIRouteRequiresBearer(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("upstream should never be called when auth fails")
	}))
	defer upstream.Close()

	srv := startTestServer(t, upstream.URL, "expected-token")
	url := "http://127.0.0.1:" + strconv.Itoa(srv.Port()) + "/v1/chat/completions"

	resp, err := http.Post(url, "application/json", bytes.NewReader([]byte(`{}`)))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("got status %d, want 401", resp.StatusCode)
	}
}

func TestOpenAIRouteRewritesAliasAndForwards(t *testing.T) {
	var gotModel string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fake-or-key" {
			t.Errorf("upstream did not receive expected Authorization header: %q", r.Header.Get("Authorization"))
		}
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		_ = json.Unmarshal(body, &payload)
		gotModel, _ = payload["model"].(string)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"hello"}}]}`))
	}))
	defer upstream.Close()

	srv := startTestServer(t, upstream.URL, "expected-token")
	url := "http://127.0.0.1:" + strconv.Itoa(srv.Port()) + "/v1/chat/completions"

	reqBody, _ := json.Marshal(map[string]any{"model": "agentroute-balanced", "messages": []any{}})
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer expected-token")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got status %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != `{"choices":[{"message":{"content":"hello"}}]}` {
		t.Fatalf("unexpected body: %s", body)
	}
	if gotModel != "openrouter/some/model" {
		t.Fatalf("upstream received model %q, want alias rewritten to %q", gotModel, "openrouter/some/model")
	}

	// Logging happens in the handler goroutine right after the response
	// finishes streaming; the client can observe EOF (length-framed
	// bodies complete by byte count, not by the handler returning)
	// fractionally before that log.Add call runs. Poll briefly rather
	// than asserting immediately.
	var entries []RequestEntry
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		entries = srv.RequestLog().Recent(1)
		if len(entries) == 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 logged request, got %d", len(entries))
	}
	if entries[0].StatusCode != http.StatusOK || entries[0].Model != "openrouter/some/model" {
		t.Fatalf("unexpected log entry: %+v", entries[0])
	}
}

func TestOpenAIRouteStreamsChunkedSSEInOrder(t *testing.T) {
	chunks := []string{
		"data: {\"choices\":[{\"delta\":{\"content\":\"hel\"}}]}\n\n",
		"data: {\"choices\":[{\"delta\":{\"content\":\"lo \"}}]}\n\n",
		"data: {\"choices\":[{\"delta\":{\"content\":\"world\"}}]}\n\n",
		"data: [DONE]\n\n",
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		// An upstream streaming response is unbounded; Go only omits
		// Content-Length automatically when nothing forces buffering. Set
		// Transfer-Encoding explicitly so this test fails the same way a
		// real SSE upstream would if hop-by-hop headers leaked through.
		w.Header().Set("Transfer-Encoding", "chunked")
		w.WriteHeader(http.StatusOK)
		flusher := w.(http.Flusher)
		for _, c := range chunks {
			_, _ = w.Write([]byte(c))
			flusher.Flush()
		}
	}))
	defer upstream.Close()

	srv := startTestServer(t, upstream.URL, "expected-token")
	url := "http://127.0.0.1:" + strconv.Itoa(srv.Port()) + "/v1/chat/completions"

	reqBody, _ := json.Marshal(map[string]any{"model": "agentroute-balanced", "stream": true})
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer expected-token")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got status %d, want 200", resp.StatusCode)
	}

	// The gateway must not forward the upstream's hop-by-hop framing
	// headers verbatim; it re-frames the response on its own connection.
	if v := resp.Header.Get("Transfer-Encoding"); v != "" {
		t.Errorf("leaked hop-by-hop Transfer-Encoding header: %q", v)
	}
	if v := resp.Header.Get("Connection"); v != "" {
		t.Errorf("leaked hop-by-hop Connection header: %q", v)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	want := ""
	for _, c := range chunks {
		want += c
	}
	if string(body) != want {
		t.Fatalf("streamed body mismatch.\ngot:  %q\nwant: %q", body, want)
	}
}

func TestOpenAIRouteUnknownAliasReturnsBadGateway(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("upstream should never be called for an unmapped alias")
	}))
	defer upstream.Close()

	srv := startTestServer(t, upstream.URL, "expected-token")
	url := "http://127.0.0.1:" + strconv.Itoa(srv.Port()) + "/v1/chat/completions"

	reqBody, _ := json.Marshal(map[string]any{"model": "agentroute-unmapped"})
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer expected-token")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("got status %d, want 502", resp.StatusCode)
	}
}

func TestHealthzDoesNotRequireAuth(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer upstream.Close()

	srv := startTestServer(t, upstream.URL, "expected-token")
	resp, err := http.Get("http://127.0.0.1:" + strconv.Itoa(srv.Port()) + "/healthz")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got status %d, want 200", resp.StatusCode)
	}
}
