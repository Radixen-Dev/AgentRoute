# Getting started

## 1. Install

**Homebrew (macOS/Linux):**

```sh
brew install --cask Radixen-Dev/agentroute/agentroute
```

**Scoop (Windows):**

```sh
scoop bucket add agentroute https://github.com/Radixen-Dev/scoop-agentroute
scoop install agentroute
```

**Build from source:**

```sh
git clone https://github.com/Radixen-Dev/AgentRoute.git
cd AgentRoute
go build -o bin/agentroute ./cmd/agentroute
```

AgentRoute also needs [LiteLLM](https://github.com/BerriAI/litellm) on `PATH` in v1 (`pipx install litellm`
is the easiest route). This is checked by `agentroute doctor` below.

## 2. Check your environment

```sh
agentroute doctor
```

This reports, pass/fail, on everything `agentroute up` needs: an OpenRouter API key, `litellm` on `PATH`,
Claude Code detected, and the gateway's port being free. Fix anything it flags before continuing — `up`
will fail with the same checks anyway, just later and with less context.

## 3. Set your OpenRouter API key

```sh
agentroute key set --value sk-or-v1-...
# or, to avoid the key touching your shell history:
echo "sk-or-v1-..." | agentroute key set --stdin
```

The key is stored in your OS keyring (Windows Credential Manager / macOS Keychain / Linux Secret Service)
where available, falling back to a `0600`-permission file otherwise. Setting the `OPENROUTER_API_KEY`
environment variable always takes precedence over whatever is stored — useful for CI or a one-off
override.

```sh
agentroute key status   # confirm it's configured, and where it came from
```

## 4. Create a profile

A *profile* is a named mapping from AgentRoute's three generic tiers (heavy/balanced/fast — Claude Code's
Opus/Sonnet/Haiku) to OpenRouter model ids:

```sh
agentroute profiles create default \
  --heavy openrouter/anthropic/claude-opus-4.5 \
  --balanced openrouter/anthropic/claude-sonnet-4.6 \
  --fast openrouter/anthropic/claude-haiku-4.5

agentroute profiles activate default
```

Not sure what model ids are available? `agentroute models` lists the live OpenRouter catalog
(`agentroute models --filter claude` to narrow it down).

You can create as many profiles as you like and switch the active one at any time with
`agentroute profiles activate <name>` — the next `agentroute up` picks it up.

## 5. Start the gateway

```sh
agentroute up
```

This is **foreground-only**: it starts the LiteLLM sidecar, starts the gateway, links Claude Code (backing
up `~/.claude/settings.json` first), and then blocks until you press Ctrl+C — at which point it unlinks
Claude Code and shuts both processes down, in that order, before exiting. Closing the terminal or killing
the process some other way skips that unwind; see [`agentroute down`](cli.md#agentroute-down) for recovery.

Leave it running in its own terminal and use `claude` as normal in another one. Its requests are now served
by whatever models your active profile assigned.

## 6. Or just run `agentroute`

Everything above (profiles, model picking, starting/stopping the gateway) has a TUI equivalent. Run
`agentroute` with no arguments in an interactive terminal and it opens the Dashboard, from which `u`
starts the gateway and `2` jumps to Profiles. `?` shows the full keymap at any time.

## Switching models without restarting Claude Code

Activate a different profile, or edit the active one's tier→model mapping, and the *next* request Claude
Code makes is served by the new model — no Claude Code restart needed. The gateway re-reads the active
profile per request, not once at startup.

## Cleaning up

`Ctrl+C` on a running `agentroute up` is the normal path and already cleans up fully. If something went
wrong (a crash, a closed terminal) and `agentroute status` reports stale state, run:

```sh
agentroute down
```

This unlinks Claude Code and clears AgentRoute's own bookkeeping, regardless of whether a gateway process
is even still running.
