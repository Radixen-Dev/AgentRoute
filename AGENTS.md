# AGENTS.md

Context for AI coding agents (and any human skimming for the fast version) working in this repo.
This file is intentionally light on implementation detail — that goes stale the moment a file moves.
Read this for orientation, then go explore the code; it's the source of truth for everything below it.

## What this project is

AgentRoute lets [Claude Code](https://claude.com/claude-code) make its model calls through
[OpenRouter](https://openrouter.ai) instead of Anthropic's API directly, so a user pays for one
OpenRouter key and can assign any OpenRouter model to Claude Code's Opus/Sonnet/Haiku tiers — without
Claude Code itself changing at all. It's a single Go binary: a small local gateway plus either an
animated terminal UI or a fully scriptable CLI to drive it.

The "why" matters more than the "how" here: this exists because Claude Code only talks to Anthropic by
default, and people want the model flexibility (and pricing) OpenRouter offers without giving up Claude
Code's UX. Every design decision in this repo traces back to that — reversibility (never leave a user's
`~/.claude/settings.json` in a broken state), and "no surprises" (everything Claude Code does is via its
own documented config hooks, never by rewriting its files with an LLM or other non-deterministic means).

Read [README.md](README.md) first for the full pitch and architecture diagram, then
[docs/concepts.md](docs/concepts.md) for the vocabulary below in depth. Both are kept current — if you
notice they aren't, that's a bug, fix it.

## Vocabulary you'll need

These terms recur throughout the codebase and the docs. Skimming this list before exploring will save
you from re-deriving it from scratch:

- **Gateway** — the local HTTP server AgentRoute runs. Claude Code is pointed at it; it authenticates
  requests, rewrites model aliases to whatever the active profile maps them to, and forwards upstream.
- **Sidecar** — a managed [LiteLLM](https://github.com/BerriAI/litellm) process the gateway proxies
  Anthropic-format requests through in v1 (the one non-Go runtime dependency; see "Known trade-offs"
  below). The gateway owns its lifecycle (start/health-check/restart).
- **Translator** — the thing that speaks one wire protocol (Anthropic, OpenAI, Gemini) on the gateway's
  inbound side. v1 only wires up Anthropic (via the sidecar); the interface is built so others slot in
  without touching the gateway core.
- **ModelRouter / tiers** — AgentRoute exposes three stable aliases (`heavy`/`balanced`/`fast`) that a
  user's active **profile** maps to real OpenRouter model IDs. Claude Code never sees a real model name —
  just the alias — so changing a profile changes routing with no Claude Code restart needed.
- **Platform adapter** — the thing that points a coding tool at the gateway and un-points it cleanly
  (`Link`/`Unlink`). Claude Code is the only one shipped (in-tree); the same shape is also expressible as
  a declarative TOML manifest (see `manifests/`) for tools that don't need custom Go logic — Codex and
  Gemini CLI exist today only as schema-validated example manifests, not enabled adapters.
- **Plain mode** — every interactive flow (TUI included) has a non-interactive CLI equivalent with
  `--json` output and stable exit codes. This is a hard architectural constraint, not a nice-to-have —
  see [docs/cli.md](docs/cli.md). If you add an interactive-only flow, you've introduced a bug.

## Where to look

Don't treat this as a map of every file — it'll be wrong within a few commits. Treat it as a starting
point for `internal/`:

| You're investigating... | Start in |
|---|---|
| The gateway, request routing, wire protocols | `internal/gateway` |
| The LiteLLM sidecar lifecycle | `internal/sidecar` |
| Claude Code wiring (the only in-tree adapter) | `internal/platform` |
| Manifest-driven adapters (Codex/Gemini examples) | `internal/platform`, `manifests/` |
| CLI commands / plain-mode contract | `internal/cli` |
| The TUI | `internal/tui` |
| OpenRouter API client, profiles | `internal/openrouter`, `internal/profile` |
| Where AgentRoute's own state/config lives on disk | `internal/paths`, `internal/config` |

The entrypoint is `cmd/agentroute/main.go`. When in doubt, `go doc` and grep beat memorized structure.

## Known trade-offs (so you don't "fix" them by accident)

- **The LiteLLM sidecar is a deliberate v1 trade-off**, not an oversight. v1 is a hybrid (native Go core,
  managed-Python-process Anthropic translation); v2 replaces it with a native Go translator. Don't treat
  "this depends on a Python tool" as a bug to silently work around — it's tracked, intentional, and
  documented in `docs/concepts.md`'s roadmap section.
- **Only Claude Code is enabled in v1.** Codex/Gemini manifests exist purely to validate the manifest
  schema generalizes — they are not wired into the runtime registry on purpose.

## Verifying your work

```sh
make build   # go build ./cmd/agentroute
make test    # go test -race ./...
make lint    # golangci-lint run (CI uses the pinned version in .golangci.yml; match it locally)
make run     # build and launch the TUI
make demo    # regenerate docs/demo/*.gif from tapes/*.tape (only if you touched a TUI screen or a tape)
```

Run `agentroute doctor` to sanity-check your own environment (LiteLLM on `PATH`, key configured, etc.)
before assuming a failure is a real bug versus a missing local dependency.

If you can't run something (no display, no Python, sandboxed shell), say so explicitly in your PR or
summary rather than claiming it passed.

## Source control rules

These apply to every contributor, human or agent. They are not suggestions — PRs that violate them will
be asked to fix up, not waved through.

**Branching & commits**
- Never push directly to `main` — it's protected and will reject it anyway. Branch off `main`, e.g.
  `feat/short-description`, `fix/short-description`, `docs/short-description`.
- One logical change per PR. Don't bundle an unrelated refactor, formatting pass, or "while I was in
  there" fix into a feature/bugfix branch — split it out.
- Commit subjects: imperative, scoped, no trailing period (`fix: handle empty profile name`, not
  `Fixed bug.`). `fix:`/`feat:`/`docs:`/`chore:`/`test:`/`refactor:` prefixes are preferred, not enforced
  by CI.
- Never force-push a shared/already-reviewed branch without calling it out to reviewers first — they may
  have local copies or pending comments tied to specific commits.
- Never commit secrets, tokens, API keys, or anything from a local AgentRoute state directory. If you're
  unsure whether something is sensitive, treat it as sensitive.
- **No AI co-authorship attribution in commits or PRs.** Commits are attributed to the human who opened
  the PR, full stop — no `Co-Authored-By` trailers naming an AI tool, no AI tool listed as a contributor
  anywhere in this repo's history. If you're an agent making the commit, the human directing you is the
  author and the one accountable for the change; say so in the PR description if it's relevant context,
  not in the commit metadata.

**Pull requests**
- Fill out [`.github/PULL_REQUEST_TEMPLATE.md`](.github/PULL_REQUEST_TEMPLATE.md) completely — the
  checklist exists because every item on it has caused a real problem before (untested TUI-only flows,
  lint failures discovered post-merge, etc.).
- A PR description should say *why*, not just *what* — the diff already shows what changed.
- Keep CI green before requesting review. A red PR wastes a reviewer's first pass.
- See [CONTRIBUTING.md](CONTRIBUTING.md) for the current required-approval count and reviewer list — it
  changes as the maintainer team grows, so it's tracked there rather than duplicated here.

**Issues**
- Use the bug report / feature request templates under `.github/ISSUE_TEMPLATE/`. Bug reports need
  reproduction steps and `agentroute doctor --json` output at minimum.
- Security vulnerabilities are **never** filed as public issues — see [SECURITY.md](SECURITY.md).

**Things that need an explicit human sign-off, not just "technically possible for an agent to do"**
- Changing branch protection, repo settings, CODEOWNERS, or any CI permission scope.
- Rewriting published git history (force-push, `filter-branch`, amending a pushed/merged commit) or
  moving/deleting an already-pushed tag.
- Anything that touches release signing, secrets, or the publish path to Homebrew/Scoop/GitHub Releases.

If a task seems to require one of the above, stop and ask rather than finding a clever way around it.

## Keeping this file current

AGENTS.md goes stale exactly like any other doc, and a stale agent-orientation doc is worse than none
(it actively misdirects). Rules for keeping it honest:

- If a PR changes something this file asserts as true (a vocabulary term, a directory's purpose, a
  hard constraint like plain-mode parity, the verification commands), update AGENTS.md **in the same
  PR**. Don't defer it to a follow-up.
- Resist adding implementation detail that belongs in `docs/` instead (exact function signatures, line
  numbers, internal data structures). This file should still be 95% accurate a year from now; `docs/`
  can churn faster.
- If you're an agent and you find this file actively wrong while working, fix it as part of your change
  — don't just work around the inaccuracy silently.
- Periodically (e.g. whenever `docs/concepts.md` gets a structural rewrite), re-read this file end to
  end and check it still matches reality.
