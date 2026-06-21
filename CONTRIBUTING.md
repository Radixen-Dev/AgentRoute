# Contributing to AgentRoute

Thanks for considering a contribution. AgentRoute uses a strict fork-based workflow.

## Workflow

1. **Fork** `Radixen-Dev/AgentRoute` to your own account. Direct pushes to `main` are disabled by branch protection — there is no other way in.
2. Branch off `main` in your fork: `git checkout -b feat/my-change`.
3. Make your change. Add or update tests. Update docs if behavior changed.
4. Open a pull request against `Radixen-Dev/AgentRoute:main`.
5. **One approval from a CODEOWNER** (currently just `@Dborasik`; see [.github/CODEOWNERS](.github/CODEOWNERS)) is required before merge, and CI must be green. The original intent was two CODEOWNERS — that goes back into effect once `@Gesso64` has collaborator access.

## Local development

Requires Go 1.23+.

```sh
make build   # build ./bin/agentroute
make test    # go test -race ./...
make lint    # golangci-lint run
make run     # build and launch the TUI
```

For the v1 Anthropic translation path you also need a `litellm` install reachable on `PATH` (`pipx install litellm`) and an `OPENROUTER_API_KEY` exported. Run `agentroute doctor` to check your environment.

## Updating the demo GIFs

`make demo` renders `tapes/*.tape` (via [VHS](https://github.com/charmbracelet/vhs)) into `docs/demo/*.gif`. CI's `demo.yml` re-renders the tapes on every push to `tapes/**` as a regression test for the render pipeline, but it only uploads the result as a build artifact — it never commits, because this repo doesn't grant Actions permission to open PRs. If a TUI screen or a tape script changes in a way that should update the GIFs, run `make demo` locally and include the regenerated files in your PR.

## Adding a platform adapter

Most tools should be addable **without writing Go code**, via a TOML manifest (see `manifests/claude-code.toml` and the worked examples in `manifests/examples/`). A manifest declares:
- how to detect the tool (`[detect]`),
- which wire protocol the gateway must serve it (`anthropic` / `openai` / `gemini`),
- the generic AgentRoute role tiers it maps to (`[roles]`: `heavy` / `balanced` / `fast` — a tool may use a subset),
- how to write that into the tool's config (`[config_target]` + `[wiring.*]`).

Only fall back to an in-tree Go adapter (`internal/platform/<name>/adapter.go`, implementing the `Platform` interface in `internal/platform/platform.go`) when the wiring needs logic the manifest schema can't express (e.g. Claude Code's JSON-merge-with-backup semantics).

See `docs/plugins.md` for the full manifest schema reference.

## Commit/PR conventions

- Conventional, imperative commit subjects (`fix:`, `feat:`, `docs:`, `chore:`) are preferred but not enforced by CI.
- Keep PRs scoped to one change. Use the PR template checklist.
- Any new interactive TUI flow must ship with a non-interactive CLI equivalent (the plain-mode contract is non-negotiable — see `docs/cli.md`).
