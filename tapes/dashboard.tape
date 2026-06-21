# VHS tape for AgentRoute's README hero demo. Rendered by
# .github/workflows/demo.yml on every push touching tapes/**, against the
# binary that workflow just built — do not render this locally unless you
# have VHS (https://github.com/charmbracelet/vhs) installed; it requires a
# real TTY VHS provides via its own pty, which this repo's dev environment
# does not assume.
#
# Boots straight to the Dashboard (no OpenRouter key or litellm needed for
# this one — it never starts the gateway), opens the help overlay to show
# the full keymap, then closes it.

Output docs/demo/dashboard.gif

Set Shell bash
Set FontSize 16
Set Width 1200
Set Height 700
Set Theme "Dracula"
Set Padding 20
Set TypingSpeed 60ms

Type "agentroute"
Enter
Sleep 2s

Type "?"
Sleep 2s

Type "?"
Sleep 1s

Type "2"
Sleep 1.5s

Escape
Sleep 1s

Type "q"
Sleep 500ms
