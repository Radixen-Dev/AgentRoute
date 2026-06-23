# AgentRoute

Route [Claude Code](https://claude.com/claude-code) through [OpenRouter](https://openrouter.ai) via a
local gateway, with either a TUI or a scriptable plain-mode CLI.

AgentRoute is a single Go binary. It does not modify Claude Code, does not rewrite `CLAUDE.md`, and does
not leave anything behind: it wires Claude Code to a local gateway using Claude Code's own documented
environment-variable hooks, and unwinds that wiring exactly on shutdown.

## Why this exists

Claude Code speaks the Anthropic Messages API. OpenRouter gives you one API key and a marketplace of
models from every provider. Without something in between, you can't point Claude Code at OpenRouter
directly — the wire formats don't match, and Claude Code has no native concept of "any OpenRouter model."

AgentRoute is that something: a gateway that authenticates Claude Code's requests, maps its three
model tiers (Opus/Sonnet/Haiku) to whichever OpenRouter models you've assigned them in your active
*profile*, and forwards the (translated) request on.

## Where to go next

- New here? Start with [Getting started](getting-started.md).
- Want the architecture, not just the steps? Read [Concepts](concepts.md).
- Looking for a specific command or exit code? See the [CLI reference](cli.md).
- Wondering whether your tool (not Claude Code) can be wired up? See [Platforms & plugins](plugins.md).
- Something not working? Check [Troubleshooting](troubleshooting.md).

## Project status

v1 ships **Claude Code only**, over **OpenRouter only**, using a managed LiteLLM sidecar for the
Anthropic↔OpenRouter translation. Both of those are deliberate v1 scope cuts, not the end state — see
[Concepts → Roadmap](concepts.md#roadmap-v2-and-beyond).
