# AgentRoute branding

This is the canonical brand guide. The palette below is **locked** (architecture plan §8) and has exactly
one source of truth in code: [`internal/tui/theme/tokens.go`](internal/tui/theme/tokens.go). A test,
`TestNoHardCodedHexOutsideTokens` in [`internal/tui/theme/tokens_test.go`](internal/tui/theme/tokens_test.go),
fails CI if any other file under `internal/tui` hard-codes a hex color instead of importing a token — so
this document and the code cannot silently drift apart.

## Name & wordmark

**AgentRoute** — set in monospace, with the "Route" half rendered in `AccentCyan`. The TUI's header and
splash screen are the canonical reference for how the wordmark is split and colored
(see [`internal/tui/header.go`](internal/tui/header.go) and [`internal/tui/splash.go`](internal/tui/splash.go)).

No final logo mark or social-card image exists yet — that's a design follow-up, not something this session
generates. If you have a screenshot or mockup you want the palette/wordmark checked against, open an issue
or hand it to whoever picks up that follow-up; nothing here should be treated as overriding a real design
asset once one exists.

## Palette

| Token | Hex | Role |
|---|---|---|
| `Ink` | `#0F1419` | Base background |
| `Surface` | `#171E25` | Header / status bar background |
| `SurfaceAlt` | `#202A33` | Card / toast background |
| `Border` | `#27343F` | Dividers, card borders |
| `AccentCyan` | `#41D6C3` | Primary accent — selections, the "Route" wordmark half, help-key hints |
| `AccentBlue` | `#7AA7FF` | Secondary accent — card titles, links, toast borders |
| `OK` | `#80DF96` | Success / "up" state |
| `Warn` | `#FFC86B` | Warnings |
| `Err` | `#FF7676` | Errors / "down" state |
| `Text` | `#E6EDF3` | Primary body text |
| `Muted` | `#7D8A99` | Secondary text, status bar hints |

Semantic mapping: primary accent is always `AccentCyan`; secondary/links are `AccentBlue`; state colors
(`OK`/`Warn`/`Err`) are reserved for exactly that — gateway/link/doctor status, never decoration. Surfaces
layer `Ink` → `Surface` → `SurfaceAlt` from furthest back to closest to the user's attention.

## Typography

Monospace-first: JetBrains Mono or any Nerd Font variant for glyph support (the TUI uses box-drawing and a
few Nerd Font icons in status pills), falling back to the system monospace font. Docs (mkdocs-material) use
the theme's default type stack — no custom web fonts are loaded.

## Voice & tone

Precise, developer-direct, lightly playful — never marketing-fluffy. Error messages always say what failed
*and* the fix (see any `cli` package error string for the pattern: `"%w; run: agentroute key set --value <key>"`).
Docs explain the *why* before the *how* where it isn't obvious (see docs/concepts.md's framing of why the
gateway sits in front of LiteLLM even in v1).

## Where this applies

The TUI, the docs site, and this README/the GitHub social preview should all read as the same product.
Concretely:

- TUI: every color from `theme.Styles`, no exceptions (enforced by the lint test above).
- mkdocs site (`mkdocs.yml`): Material theme, `slate` scheme, `teal` primary / `indigo` accent — the
  closest stock Material palette to `AccentCyan`/`AccentBlue` until a custom CSS palette is worth the
  upkeep.
- README/social card: same teal/indigo/dark family; no separate palette.
