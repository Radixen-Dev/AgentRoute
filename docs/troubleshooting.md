# Troubleshooting

Run `agentroute doctor` first — it checks the four things most issues below trace back to (OpenRouter key,
`litellm` on `PATH`, Claude Code detected, gateway port free) and tells you exactly what's wrong and how to
fix it. The sections below cover what to do when `doctor` passes but something still doesn't work, or to
explain *why* a given check exists.

## "no OpenRouter API key configured" (exit code 3)

`agentroute key status` shows whether a key is configured and where it's coming from. Precedence is: the
`OPENROUTER_API_KEY` environment variable (if set, always wins) → OS keyring → `0600` file fallback. If
you've set the env var and AgentRoute still reports no key, double check it's exported in the *same* shell
you're running `agentroute` from — a key set in one terminal tab doesn't propagate to another.

## "litellm: not found on PATH"

v1's gateway uses a managed [LiteLLM](https://github.com/BerriAI/litellm) subprocess for the Anthropic↔
OpenRouter translation (see [Concepts](concepts.md#sidecar) for why). Install it with `pipx install
litellm` (or `pip install litellm` if you don't use pipx) and confirm with `litellm --version`.

## "port N is already in use"

Something else is already listening on AgentRoute's configured port (default `4505`). Either stop that
process, or pass a different port: `agentroute up --port 4510`. If it's a *previous* `agentroute up` that
didn't shut down cleanly, see the stale-state section below instead of just picking a new port — the old
one is probably still holding a stale link in `~/.claude/settings.json`.

## `agentroute status` reports stale state

```
stale state found (recorded port N is not responding) — a previous `up` may have crashed.
```

This means AgentRoute's own bookkeeping says a gateway should be running, but nothing answers its health
check — most likely the process crashed or the machine was shut down without a clean Ctrl+C. Run:

```sh
agentroute down
```

This unlinks Claude Code (so it stops pointing at a gateway that no longer exists) and clears the stale
bookkeeping, regardless of whether any AgentRoute process is actually still running. It's always safe to
run, even if there's nothing to clean up.

## Claude Code isn't picking up the new model after switching profiles

It should, without a restart — the gateway resolves the active profile per request, not once at startup.
If it isn't:

1. Confirm the profile you activated is the one you expect: `agentroute profiles list` marks the active
   one with `*`.
2. Confirm the gateway that's actually running was started *after* you activated it, or pass
   `--profile <name>` explicitly to `agentroute up` for that run.
3. Check `agentroute status --json` for the `profile` field actually in use by the running gateway.

## `~/.claude/settings.json` looks wrong after `agentroute down` / Ctrl+C

This should never happen — `Unlink` restores the file to a byte-identical copy of its pre-`Link` state (a
backup is taken the first time `Link` runs). If you find it doesn't:

1. Check for a `settings.json.agentroute.bak` (or similar) backup file next to it — `Unlink`'s fallback
   path uses recorded key names if the backup file is missing, which only happens if something deleted it
   out-of-band between `Link` and `Unlink`.
2. This is a bug if it happens with the backup file intact — please file an issue with the *exact*
   before/after content (redact your API keys/tokens first) at
   <https://github.com/Radixen-Dev/AgentRoute/issues>.

## The TUI looks broken / colors are wrong

Set `NO_COLOR=1` or `AGENTROUTE_PLAIN=1` to force plain output and confirm the underlying command still
works — if it does, the issue is terminal/color-profile detection, not the gateway. `AGENTROUTE_REDUCE_MOTION=1`
disables animations specifically (useful over a slow SSH link) without going fully plain. Please include
your terminal emulator and `$TERM` value in any bug report about TUI rendering.

## Still stuck?

Open an issue at <https://github.com/Radixen-Dev/AgentRoute/issues> with the output of
`agentroute doctor` and `agentroute version` — see [SECURITY.md](https://github.com/Radixen-Dev/AgentRoute/blob/main/SECURITY.md)
instead if it's a security issue, not a regular bug.
