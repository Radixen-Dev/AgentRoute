# Branding

The canonical brand guide lives at [`BRANDING.md`](https://github.com/Radixen-Dev/AgentRoute/blob/main/BRANDING.md)
in the repo root, not here — this page exists only so it shows up in the docs site nav. It covers the
locked palette (sourced straight from [`internal/tui/theme/tokens.go`](https://github.com/Radixen-Dev/AgentRoute/blob/main/internal/tui/theme/tokens.go)),
typography, voice/tone, and where each of those applies (TUI, this docs site, the README/social card).

## Palette at a glance

| Token | Hex | Role |
|---|---|---|
| `Ink` | `#0F1419` | Base background |
| `Surface` | `#171E25` | Header / status bar background |
| `SurfaceAlt` | `#202A33` | Card / toast background |
| `Border` | `#27343F` | Dividers, card borders |
| `AccentCyan` | `#41D6C3` | Primary accent |
| `AccentBlue` | `#7AA7FF` | Secondary accent / links |
| `OK` | `#80DF96` | Success / "up" state |
| `Warn` | `#FFC86B` | Warnings |
| `Err` | `#FF7676` | Errors / "down" state |
| `Text` | `#E6EDF3` | Primary body text |
| `Muted` | `#7D8A99` | Secondary text |

This table is a convenience copy. If it ever disagrees with `BRANDING.md` or `tokens.go`, those two win —
`tokens.go` is enforced by a test (`TestNoHardCodedHexOutsideTokens`); this page isn't.
