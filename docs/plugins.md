# Platforms & plugins

A `Platform` (`internal/platform`) is the extension boundary between AgentRoute's gateway and one specific
coding-agent tool: it knows how to detect the tool, point it at the gateway, undo that, and report whether
it's currently pointed there.

```go
type Platform interface {
    ID() string
    DisplayName() string
    Wire() gateway.Wire
    Roles() []Role
    Detect(ctx context.Context) (Detection, error)
    Link(ctx context.Context, in LinkInput) (LinkResult, error)
    Unlink(ctx context.Context) error
    Status(ctx context.Context) (LinkStatus, error)
}
```

`Link` and `Unlink` must be exact inverses: `Unlink` after `Link` restores the tool's config to
byte-identical its pre-`Link` state. `Roles` reports which of AgentRoute's generic tiers (`heavy`,
`balanced`, `fast`) the tool exposes — a tool is free to expose fewer than three.

## Claude Code (in-tree, v1)

`internal/platform/claudecode` is the only adapter actually registered in v1. It's in-tree (Go code, not a
manifest) because its wiring — a JSON merge into `~/.claude/settings.json`'s `"env"` block, with
backup/exact-restore semantics — needs more than the generic manifest interpreter (below) currently
supports. See [Concepts → What Link actually changes](concepts.md#what-link-actually-changes) for exactly
which keys it sets.

## Manifest-driven adapters

Most tools don't need custom Go code at all — just a TOML file describing how to detect the tool and how
to point it at the gateway. `internal/platform/manifest.go` defines the schema and a generic
`ManifestAdapter` that implements `Platform` purely by interpreting one.

```toml
id            = "codex"
display_name  = "Codex CLI"
wire          = "openai"

[detect]
binary        = "codex"
config_paths  = ["~/.codex/config.toml"]

[config_target]
type          = "toml"
path          = "~/.codex/config.toml"

[roles]
balanced      = "agentroute-balanced"

[wiring.toml]
"model_providers.agentroute.base_url" = "{{gateway_url}}/v1"
"model_providers.agentroute.env_key"  = "AGENTROUTE_TOKEN"
"model_provider"                      = "agentroute"
"model"                               = "{{roles.balanced}}"
```

`{{gateway_url}}`, `{{auth_token}}`, and `{{roles.<tier>}}` are the only template placeholders
`ManifestAdapter` understands; an unknown placeholder is a parse-time error, not a silent pass-through.

### `config_target.type` — what v1 actually supports

| Type | v1 status | Behavior |
|---|---|---|
| `toml` | **Implemented** | `Link` merges `[wiring.toml]`'s dotted keys into the target TOML file (creating it, and intermediate tables, as needed), taking a backup first; `Unlink` restores it. |
| `shell-env` | **Implemented, deliberately inert** | `Link` does **not** write anything — see below. |
| `json-env` | **Recognized, unsupported** | `ParseManifest`/`Validate` return `ErrUnsupportedConfigTarget`; the registry logs and skips any such manifest rather than failing to load every manifest in the directory. This is exactly Claude Code's case, which is why `manifests/claude-code.toml` exists purely as a schema reference (see its own header comment) and is never loaded — the in-tree adapter above serves Claude Code instead. |

### shell-env wiring is deliberately unimplemented

`config_target.type = "shell-env"` means the tool reads its configuration from process environment
variables — there is no file to edit. What `Link` should actually *do* about that (write a `.env` file the
user sources? export into the current shell, which a subprocess can't do to its parent? launch the tool
itself with an augmented environment?) is an open product question, not a technical one, and guessing at
it silently would be worse than doing nothing. So in v1, `Link` on shell-env wiring renders the template
values and returns them in `LinkResult.KeysSet` without touching the filesystem; `Status` always reports
not-linked (there's nothing on disk to check); `Unlink` is a no-op. `manifests/examples/gemini-cli.toml.example`
documents this same open question for the Gemini CLI manifest specifically.

### What's shipped vs. enabled

`manifests/claude-code.toml` and everything under `manifests/examples/` (`codex.toml.example`,
`gemini-cli.toml.example`) are **schema references with unit-test coverage, not enabled adapters** — the
registry (`internal/platform/registry.go`) only loads `*.toml` files directly inside `manifests/`,
explicitly excluding the `examples/` subdirectory and anything ending `.example`. In the shipped
`manifests/` directory, that nets out to **zero** manifest adapters loaded (claude-code.toml is skipped as
json-env), so v1's actual registered platform list is exactly `[claude-code]` — the in-tree adapter.

Enabling Codex or Gemini CLI for real is a matter of: dropping a real (non-`.example`) manifest into
`manifests/`, wiring it into whichever call site currently constructs `claudecode.New()` directly
(`internal/cli/platforms.go`, `internal/orchestrator/orchestrator.go`, `internal/tui/services.go`) via
`platform.NewRegistry` instead, and — for Gemini specifically — resolving the shell-env question above
first.

## v2: out-of-process plugins (not implemented)

For adapter or translator logic that genuinely needs real code — not declarative wiring — v2 plans a gRPC
plugin protocol via [`github.com/hashicorp/go-plugin`](https://github.com/hashicorp/go-plugin): an
out-of-process plugin binary implements `Detect`/`Link`/`Unlink`/`Status` (and optionally a custom
`Translator`) over gRPC, in any language `go-plugin` supports a client for. See
[`plugins/PROTOCOL.md`](https://github.com/Radixen-Dev/AgentRoute/blob/main/plugins/PROTOCOL.md) for the planned proto shape. This is explicitly a v1
non-goal — nothing under `plugins/` is loaded or called by anything in v1.
