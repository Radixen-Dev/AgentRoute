// SPDX-License-Identifier: GPL-3.0-only

package gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OpenAINativeTranslator forwards OpenAI Chat Completions-wire requests
// (as sent by tools like Codex) directly to OpenRouter, which is itself
// OpenAI-compatible. Unlike the v1 Anthropic path, this needs no sidecar:
// it is the proof that AgentRoute's native (no-LiteLLM) translation path
// works end to end, and the model the v2 native Anthropic translator will
// follow.
type OpenAINativeTranslator struct {
	// HTTPClient is used to call upstream; defaults to http.DefaultClient
	// when nil (set in Handler).
	HTTPClient *http.Client
}

// Wire implements Translator.
func (t *OpenAINativeTranslator) Wire() Wire { return WireOpenAI }

// Handler implements Translator. The returned handler expects to be
// mounted at a path tools POST their OpenAI-wire chat completion requests
// to (conventionally "/v1/chat/completions").
func (t *OpenAINativeTranslator) Handler(upstream Upstream, router ModelRouter, log *RequestLog) http.Handler {
	client := t.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read request body", http.StatusBadRequest)
			return
		}

		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			http.Error(w, "request body is not valid JSON", http.StatusBadRequest)
			return
		}

		alias, _ := payload["model"].(string)
		model, ok := router.Resolve(alias)
		if !ok {
			msg := fmt.Sprintf("no model mapped for alias %q in the active profile", alias)
			http.Error(w, msg, http.StatusBadGateway)
			if log != nil {
				log.Add(RequestEntry{Time: start, Wire: WireOpenAI, Alias: alias, StatusCode: http.StatusBadGateway, Duration: time.Since(start), Err: msg})
			}
			return
		}
		payload["model"] = model

		rewritten, err := json.Marshal(payload)
		if err != nil {
			http.Error(w, "failed to rewrite request body", http.StatusInternalServerError)
			return
		}

		upstreamURL := strings.TrimRight(upstream.BaseURL, "/") + "/chat/completions"
		upstreamReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, upstreamURL, bytes.NewReader(rewritten))
		if err != nil {
			http.Error(w, "failed to build upstream request", http.StatusInternalServerError)
			return
		}
		upstreamReq.Header.Set("Content-Type", "application/json")
		upstreamReq.Header.Set("Authorization", "Bearer "+upstream.APIKey)

		resp, err := client.Do(upstreamReq)
		if err != nil {
			msg := fmt.Sprintf("upstream request failed: %v", err)
			http.Error(w, msg, http.StatusBadGateway)
			if log != nil {
				log.Add(RequestEntry{Time: start, Wire: WireOpenAI, Alias: alias, Model: model, StatusCode: http.StatusBadGateway, Duration: time.Since(start), Err: msg})
			}
			return
		}
		defer func() { _ = resp.Body.Close() }()

		copyResponseHeaders(w.Header(), resp.Header)
		w.WriteHeader(resp.StatusCode)

		flusher, canFlush := w.(http.Flusher)
		buf := make([]byte, 4096)
		for {
			n, readErr := resp.Body.Read(buf)
			if n > 0 {
				if _, writeErr := w.Write(buf[:n]); writeErr != nil {
					break
				}
				if canFlush {
					flusher.Flush()
				}
			}
			if readErr != nil {
				break
			}
		}

		if log != nil {
			log.Add(RequestEntry{Time: start, Wire: WireOpenAI, Alias: alias, Model: model, StatusCode: resp.StatusCode, Duration: time.Since(start)})
		}
	})
}
