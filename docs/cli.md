# CLI reference

## Plain-mode contract

AgentRoute has two front ends over the same internal packages: the TUI, and this plain CLI. The CLI is
first-class and non-negotiable as an automation surface — every interactive flow has a flag-driven
equivalent, and the following rules are stable:

- **No TTY control.** No color, animation, spinners, or alternate screen, ever, from a plain command.
- **`--json`** (a persistent flag on every command) emits newline-delimited JSON on **stdout**. Anything
  that isn't the structured result — progress, warnings — goes to **stderr**, so piping stdout to `jq`
  never sees noise mixed in.
- **Stable exit codes.** Scripts and other agents may depend on these:

  | Code | Meaning |
  |---|---|
  | `0` | OK |
  | `1` | Generic error |
  | `2` | Usage error (bad flags/args) |
  | `3` | Missing or invalid OpenRouter API key |
  | `4` | Gateway or sidecar failed to start/stay up |
  | `5` | Platform link/unlink failed |

- **No prompts.** A command that's missing required input exits `2` with a message naming the flag to
  pass — it never blocks waiting for interactive input.

## `agentroute` (no subcommand)

Launches the TUI if stdout is an interactive terminal; otherwise prints help (same as `--help`). Set
`AGENTROUTE_PLAIN=1` to force the help-text behavior even on a TTY — useful for a script that sometimes
runs interactively and should never accidentally launch a full-screen UI.

## `agentroute up`

Starts the gateway and the LiteLLM sidecar, links Claude Code, then **blocks in the foreground** until
Ctrl+C, unlinking and shutting both down cleanly on the way out. AgentRoute does not run as a background
daemon in v1 — see [`agentroute down`](#agentroute-down) for the crash-recovery path.

| Flag | Default | Meaning |
|---|---|---|
| `--profile` | active profile | Profile to use for this run, without changing which one is active |
| `--port` | configured default (4505) | Gateway port; `0` means "use the configured default" |
| `--no-link` | `false` | Start the gateway and sidecar but don't link Claude Code |

Exits `3` if no OpenRouter key is configured, `4` if the gateway or sidecar fails to start (or exits
unexpectedly while running), `5` if linking Claude Code fails, `2` if no profile is active and none was
passed via `--profile`.

## `agentroute down`

Recovers from an unclean shutdown (crash, closed terminal): unlinks Claude Code and clears AgentRoute's
own stale-state bookkeeping, regardless of whether a gateway process is still actually running. Idempotent
— safe to run even if there's nothing to clean up.

## `agentroute status`

Reports whether a gateway started by `up` is still alive (it health-checks the recorded port, so a crashed
process is correctly reported as not running even though state was recorded), which port/profile it's
using, and when it started.

## `agentroute profiles`

| Subcommand | Args | Flags | Meaning |
|---|---|---|---|
| `list` | — | — | List saved profiles; marks the active one |
| `create` | `<name>` | `--heavy`, `--balanced`, `--fast` (OpenRouter model ids; at least one required) | Create or overwrite a profile |
| `delete` | `<name>` | — | Delete a saved profile |
| `activate` | `<name>` | — | Set the profile `up` uses when `--profile` is omitted |

## `agentroute models`

Lists the live OpenRouter model catalog (id, name, context length). Requires an OpenRouter key configured
(exit `3` if not). `--filter <substring>` matches against id or name, case-insensitively.

## `agentroute key`

| Subcommand | Flags | Meaning |
|---|---|---|
| `set` | `--value <key>` or `--stdin` (mutually exclusive) | Store an OpenRouter API key |
| `clear` | — | Remove the stored key from both the keyring and the file fallback |
| `status` | — | Show whether a key is configured, and its source (`env`/`keyring`/`file-fallback`/`none`) |

Storage precedence: the `OPENROUTER_API_KEY` environment variable always wins if set; otherwise the OS
keyring (Credential Manager / Keychain / Secret Service); otherwise a `0600` file fallback. `key set`
writes to the keyring if available, the file fallback otherwise — it never sets an environment variable
(that's your shell's job, and would only last the current process anyway).

Not in the original architecture plan's command list — added because plain mode needs *some*
non-interactive way to get a key into storage, and every other flow assumes one already exists.

## `agentroute link <platform>`

Points `<platform>` (`claude-code` is the only value v1 supports) at a gateway that's **already running**
— it reads the running gateway's recorded port/token and active profile rather than starting anything
itself. Requires `agentroute up` running in another terminal; exits `4` if no live gateway is found, `5`
if the link itself fails.

## `agentroute unlink <platform>`

Restores `<platform>`'s config to its pre-`Link` state. Exits `5` on failure.

## `agentroute doctor`

Runs the same checks `up` depends on and reports pass/fail for each: OpenRouter key configured, `litellm`
on `PATH`, Claude Code detected, gateway port free. Exits `1` if any check fails (after printing all of
them, not just the first failure).

## `agentroute tui`

Forces the TUI regardless of TTY detection — an escape hatch for terminals `agentroute`'s TTY check
doesn't recognize, or for scripts/aliases that want the explicit subcommand.

## `agentroute version`

Prints the version, commit, and build date injected at build time via `-ldflags`
(`dev`/`none`/`unknown` in a `go build` without them).
