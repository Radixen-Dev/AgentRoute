// SPDX-License-Identifier: GPL-3.0-only

// Package gateway implements AgentRoute's local routing gateway: it listens
// on 127.0.0.1, accepts requests in whatever wire format a linked coding
// tool speaks (Anthropic Messages, OpenAI Chat Completions, Gemini
// generateContent), rewrites the requested model alias to the OpenRouter
// model the user chose for that tier, and forwards upstream.
package gateway

import "net/http"

// Wire identifies which API wire format a Translator serves.
type Wire string

const (
	WireAnthropic Wire = "anthropic"
	WireOpenAI    Wire = "openai"
	WireGemini    Wire = "gemini"
)

// Upstream describes the OpenRouter endpoint requests are forwarded to.
type Upstream struct {
	// BaseURL is OpenRouter's API base, e.g. "https://openrouter.ai/api/v1".
	BaseURL string
	// APIKey is the user's OPENROUTER_API_KEY.
	APIKey string
}

// ModelRouter resolves an AgentRoute tier alias (e.g. "agentroute-balanced")
// to the concrete upstream model id the active profile maps it to (e.g.
// "anthropic/claude-sonnet-4.5"). ok is false if alias is not mapped by the
// active profile.
type ModelRouter interface {
	Resolve(alias string) (upstreamModel string, ok bool)
}

// MapRouter is the simplest ModelRouter: a static alias -> model map built
// from the active profile's tier assignments.
type MapRouter map[string]string

// Resolve implements ModelRouter.
func (m MapRouter) Resolve(alias string) (string, bool) {
	model, ok := m[alias]
	return model, ok
}

// Translator serves inbound requests of one wire format, translating and
// forwarding them to upstream after resolving the requested model alias
// via router. Implementations must support streaming (SSE) responses.
type Translator interface {
	Wire() Wire
	Handler(upstream Upstream, router ModelRouter, log *RequestLog) http.Handler
}
