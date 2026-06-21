# Concepts

## The pieces

```
 Claude Code  ──Anthropic /v1/messages──▶  AgentRoute gateway  ──proxy──▶  LiteLLM sidecar  ──▶  OpenRouter
(~/.claude/settings.json                  (127.0.0.1:4505,                (renders config from
 "env" block points here)                  authenticates, applies          your active profile)
                                            your tier→model mapping)
```

### Gateway

A local HTTP server (`internal/gateway`), listening on `127.0.0.1` only — never `0.0.0.0` — on a port
recorded in `agentroute status`. It:

1. Authenticates every request against a per-session random bearer token (the same value written into
   Claude Code's `ANTHROPIC_AUTH_TOKEN`).
2. Detects which wire protocol an inbound request uses by its route (`/v1/messages*` is Anthropic;
   `/v1/chat/completions` is OpenAI; Gemini's route is reserved for when a Gemini adapter ships) and hands
   it to the matching **Translator**.
3. Applies the **ModelRouter**: rewrites the request's model alias (`agentroute-heavy`, `-balanced`,
   `-fast`) to whatever OpenRouter model id the active profile assigns that tier.
4. Streams the response back unchanged (SSE pass-through).
5. Logs the request (alias, mapped model, status, latency) into a ring buffer the TUI's Dashboard and Live
   Log screens read from.

### Translator

```go
type Translator interface {
    Wire() Wire
    Handler(upstream Upstream, router ModelRouter) http.Handler
}
```

v1 ships two: `AnthropicLiteLLMTranslator`, a reverse proxy to the managed LiteLLM sidecar (this is what
actually serves Claude Code in v1), and `OpenAINativeTranslator`, a native Go forwarder to OpenRouter's
OpenAI-compatible `/v1/chat/completions` — proof that the gateway doesn't *need* LiteLLM for every wire
format, which is the seam v2's native Anthropic translator drops into.

### Sidecar

A managed [LiteLLM](https://github.com/BerriAI/litellm) subprocess (`internal/sidecar`). AgentRoute renders
its config from the active profile, starts it, polls its health endpoint, and restarts it on crash. This
is the **one Python dependency** of v1 — the explicit trade-off of shipping fast with a hybrid
gateway instead of writing a native Anthropic↔OpenRouter translator first. `agentroute doctor` checks for
it; v2 removes the dependency entirely (see [Roadmap](#roadmap-v2-and-beyond)).

### ModelRouter

Maps AgentRoute's three stable aliases to the concrete OpenRouter model the **active profile** currently
assigns:

```go
type ModelRouter interface { Resolve(alias string) (upstreamModel string, ok bool) }
```

The aliases themselves (`agentroute-heavy`/`-balanced`/`-fast`) never change — what changes is which
OpenRouter model each one currently points at, which is exactly what a *profile* records.

### Profiles

A profile (`internal/profile`) is a named `{heavy, balanced, fast} → openrouter/<model-id>` mapping,
persisted as JSON under AgentRoute's own state directory (never inside `~/.claude`). Exactly one profile
is "active" at a time (`agentroute profiles activate <name>`); `agentroute up --profile <name>` overrides
the active one for a single run without changing which one is active.

### Platform adapters

A `Platform` (`internal/platform`) is whatever knows how to point one specific tool at the gateway and
undo it — see [Platforms & plugins](plugins.md) for the interface, the in-tree Claude Code adapter, and
the manifest-driven path for adding new tools without writing Go.

## Why the gateway sits in front of LiteLLM, even in v1

The gateway — not LiteLLM — owns auth, the alias→model mapping, request logging for the TUI, and a stable
port. That means swapping LiteLLM for a native v2 translator changes nothing on Claude Code's side: same
`ANTHROPIC_BASE_URL`, same token, same aliases. The sidecar is an implementation detail of how the gateway
currently serves the Anthropic wire — never something Claude Code talks to directly.

## What Link actually changes

For Claude Code, `Link` adds five keys to the `"env"` block of `~/.claude/settings.json`:
`ANTHROPIC_BASE_URL`, `ANTHROPIC_AUTH_TOKEN`, and the three `ANTHROPIC_DEFAULT_{OPUS,SONNET,HAIKU}_MODEL`
selectors (the last three hold the AgentRoute aliases, not real model ids). Nothing else in the file is
touched, and `CLAUDE.md` is never rewritten — that was the source-of-inspiration project's approach
(an LLM rewriting your routing table inside a markdown file) and AgentRoute deliberately doesn't do that;
it's non-deterministic and one bad rewrite corrupts a file you didn't ask it to touch.

`Unlink` restores the file to a byte-identical copy of what it was before `Link` ever ran (a backup is
taken on first link), or — if that backup is somehow gone — falls back to surgically removing exactly the
five keys `Link` recorded setting. Either way, nothing AgentRoute didn't add is ever touched.

## Roadmap (v2 and beyond)

- **Native Anthropic translator** — replace the LiteLLM sidecar with native Go Anthropic↔OpenRouter
  translation (SSE, tool_use blocks, `count_tokens`, vision). Drops the Python dependency entirely.
- **Codex and Gemini CLI adapters enabled** — the manifest schema and a generic interpreter already exist
  (`internal/platform`); the example manifests in `manifests/examples/` are schema-validated but not
  registered. Enabling them is mostly a UX decision for Codex (TOML wiring, already implemented) and an
  open design question for Gemini CLI (shell-env wiring, deliberately unimplemented — see
  [Platforms & plugins](plugins.md#shell-env-wiring-is-deliberately-unimplemented)).
- **Out-of-process plugins** for adapters or translators that need real code in another language —
  see [Platforms & plugins → v2 plugin protocol](plugins.md#v2-out-of-process-plugins-not-implemented).
- **Multi-provider upstreams** beyond OpenRouter (direct OpenAI, Anthropic, Azure, Ollama).
- Per-project profiles, cost dashboards, optional background daemon/autostart.

None of the above is implemented in v1. This list exists so "is X planned?" has an answer instead of a
guess.
