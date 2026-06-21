// SPDX-License-Identifier: GPL-3.0-only

package gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"time"
)

// AnthropicLiteLLMTranslator serves Anthropic Messages-wire requests (as
// sent by Claude Code) by rewriting the model alias and reverse-proxying to
// a managed LiteLLM sidecar, which holds the OpenRouter credentials and
// performs the actual Anthropic<->OpenAI translation. This is the v1
// hybrid-engine trade-off described in the architecture plan §5.2; v2
// replaces it with a native Go Anthropic translator and this file goes
// away.
//
// Unlike OpenAINativeTranslator, this translator never talks to upstream
// (OpenRouter) directly — Upstream is ignored. The sidecar is the only
// holder of the OpenRouter key.
type AnthropicLiteLLMTranslator struct {
	// SidecarURL is the LiteLLM sidecar's base URL, e.g.
	// "http://127.0.0.1:4506".
	SidecarURL string
	// SidecarToken is the bearer credential the sidecar requires (LiteLLM's
	// general_settings.master_key, rendered by sidecar.RenderConfig).
	SidecarToken string
}

// Wire implements Translator.
func (t *AnthropicLiteLLMTranslator) Wire() Wire { return WireAnthropic }

// Handler implements Translator. The returned handler expects to be
// mounted at the Anthropic Messages routes (conventionally
// "/v1/messages" and "/v1/messages/count_tokens").
func (t *AnthropicLiteLLMTranslator) Handler(_ Upstream, router ModelRouter, log *RequestLog) http.Handler {
	target, err := url.Parse(t.SidecarURL)
	if err != nil {
		// SidecarURL is set once at startup from a value AgentRoute itself
		// constructed (127.0.0.1:<port>); a parse failure here means a
		// programming error, not a runtime condition to recover from.
		panic(fmt.Sprintf("gateway: invalid sidecar URL %q: %v", t.SidecarURL, err))
	}

	proxy := &httputil.ReverseProxy{
		// Flush immediately rather than buffering: Claude Code's streaming
		// responses (SSE) must arrive incrementally, not all at once when
		// the upstream finishes.
		FlushInterval: -1,
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			pr.Out.Host = target.Host
			pr.Out.Header.Set("Authorization", "Bearer "+t.SidecarToken)
		},
		// Set once: a single *ReverseProxy is shared across every request
		// this handler serves, so ErrorHandler/ModifyResponse must not be
		// reassigned per-request (concurrent requests would race on those
		// fields). Status is captured instead via statusRecorder below.
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			http.Error(w, fmt.Sprintf("sidecar request failed: %v", err), http.StatusBadGateway)
		},
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
				log.Add(RequestEntry{Time: start, Wire: WireAnthropic, Alias: alias, StatusCode: http.StatusBadGateway, Duration: time.Since(start), Err: msg})
			}
			return
		}
		payload["model"] = model

		rewritten, err := json.Marshal(payload)
		if err != nil {
			http.Error(w, "failed to rewrite request body", http.StatusInternalServerError)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(rewritten))
		r.ContentLength = int64(len(rewritten))
		r.Header.Set("Content-Length", strconv.Itoa(len(rewritten)))

		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		proxy.ServeHTTP(rec, r)

		if log != nil {
			log.Add(RequestEntry{Time: start, Wire: WireAnthropic, Alias: alias, Model: model, StatusCode: rec.status, Duration: time.Since(start)})
		}
	})
}

// statusRecorder wraps a ResponseWriter to capture the status code written
// by httputil.ReverseProxy (which always calls WriteHeader explicitly,
// whether forwarding the upstream's status or via ErrorHandler's
// http.Error), without touching shared *ReverseProxy state per request.
// It forwards Flush so SSE streaming (FlushInterval: -1 above) still works.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
