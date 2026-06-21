# Security Policy

## Reporting a vulnerability

Please do **not** open a public GitHub issue for security vulnerabilities.

Instead, report privately via GitHub's [Security Advisories](https://github.com/Radixen-Dev/AgentRoute/security/advisories/new) for this repository, or email the maintainers listed in [CODEOWNERS](.github/CODEOWNERS).

Include:
- A description of the vulnerability and its impact.
- Steps to reproduce (proof-of-concept welcome).
- The AgentRoute version and OS affected.

We aim to acknowledge reports within 5 business days.

## Scope

AgentRoute runs a local-only gateway (bound to `127.0.0.1`) that holds your `OPENROUTER_API_KEY` and reversibly edits coding-tool config files (e.g. `~/.claude/settings.json`). Reports involving credential handling, the local gateway's auth, or config-file edit/restore logic are especially welcome.
