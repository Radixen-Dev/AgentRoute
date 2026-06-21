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

func startAnthropicTestServer(t *testing.T, sidecarURL, sidecarToken, gatewayToken string) *Server {
	t.Helper()
	srv, err := New(Config{
		Port:  0,
		Token: gatewayToken,
		Router: MapRouter{
			"agentroute-balanced": "agentroute-balanced", // sidecar's own model_list key
		},
		Translators: []Translator{
			&AnthropicLiteLLMTranslator{SidecarURL: sidecarURL, SidecarToken: sidecarToken},
		},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	go func() { _ = srv.Serve() }()
	t.Cleanup(func() { _ = srv.Shutdown(2 * time.Second) })
	return srv
}

func TestAnthropicRouteRewritesModelAndAuthenticatesToSidecar(t *testing.T) {
	var gotModel, gotAuth string
	sidecar := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		_ = json.Unmarshal(body, &payload)
		gotModel, _ = payload["model"].(string)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"msg_1","content":[{"type":"text","text":"hi"}]}`))
	}))
	defer sidecar.Close()

	srv := startAnthropicTestServer(t, sidecar.URL, "sidecar-secret", "gw-token")
	url := "http://127.0.0.1:" + strconv.Itoa(srv.Port()) + "/v1/messages"

	reqBody, _ := json.Marshal(map[string]any{"model": "agentroute-balanced", "messages": []any{}})
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer gw-token")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got status %d, want 200", resp.StatusCode)
	}
	if gotModel != "agentroute-balanced" {
		t.Fatalf("sidecar received model %q, want %q", gotModel, "agentroute-balanced")
	}
	if gotAuth != "Bearer sidecar-secret" {
		t.Fatalf("sidecar received Authorization %q, want %q", gotAuth, "Bearer sidecar-secret")
	}

	var entries []RequestEntry
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		entries = srv.RequestLog().Recent(1)
		if len(entries) == 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if len(entries) != 1 || entries[0].StatusCode != http.StatusOK {
		t.Fatalf("unexpected log entries: %+v", entries)
	}
}

func TestAnthropicRouteCountTokensSubpathReachesHandler(t *testing.T) {
	sidecar := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages/count_tokens" {
			t.Errorf("sidecar received path %q, want %q", r.URL.Path, "/v1/messages/count_tokens")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"input_tokens":42}`))
	}))
	defer sidecar.Close()

	srv := startAnthropicTestServer(t, sidecar.URL, "sidecar-secret", "gw-token")
	url := "http://127.0.0.1:" + strconv.Itoa(srv.Port()) + "/v1/messages/count_tokens"

	reqBody, _ := json.Marshal(map[string]any{"model": "agentroute-balanced"})
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer gw-token")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got status %d, want 200", resp.StatusCode)
	}
}

func TestAnthropicRouteStreamsSSEInOrderWithoutHopByHopLeak(t *testing.T) {
	chunks := []string{
		"event: message_start\ndata: {}\n\n",
		"event: content_block_delta\ndata: {\"delta\":\"hel\"}\n\n",
		"event: content_block_delta\ndata: {\"delta\":\"lo\"}\n\n",
		"event: message_stop\ndata: {}\n\n",
	}
	sidecar := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Transfer-Encoding", "chunked")
		w.WriteHeader(http.StatusOK)
		flusher := w.(http.Flusher)
		for _, c := range chunks {
			_, _ = w.Write([]byte(c))
			flusher.Flush()
		}
	}))
	defer sidecar.Close()

	srv := startAnthropicTestServer(t, sidecar.URL, "sidecar-secret", "gw-token")
	url := "http://127.0.0.1:" + strconv.Itoa(srv.Port()) + "/v1/messages"

	reqBody, _ := json.Marshal(map[string]any{"model": "agentroute-balanced", "stream": true})
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer gw-token")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got status %d, want 200", resp.StatusCode)
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

func TestAnthropicRouteUnknownAliasNeverReachesSidecar(t *testing.T) {
	sidecar := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("sidecar should never be called for an unmapped alias")
	}))
	defer sidecar.Close()

	srv := startAnthropicTestServer(t, sidecar.URL, "sidecar-secret", "gw-token")
	url := "http://127.0.0.1:" + strconv.Itoa(srv.Port()) + "/v1/messages"

	reqBody, _ := json.Marshal(map[string]any{"model": "agentroute-unmapped"})
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer gw-token")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("got status %d, want 502", resp.StatusCode)
	}
}

func TestAnthropicRouteSidecarDownReturnsBadGateway(t *testing.T) {
	// Bind and immediately close a listener to get a port nothing is
	// listening on, simulating a sidecar that crashed or never started.
	deadURL := "http://127.0.0.1:1"

	srv := startAnthropicTestServer(t, deadURL, "sidecar-secret", "gw-token")
	url := "http://127.0.0.1:" + strconv.Itoa(srv.Port()) + "/v1/messages"

	reqBody, _ := json.Marshal(map[string]any{"model": "agentroute-balanced"})
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer gw-token")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("got status %d, want 502", resp.StatusCode)
	}
}
