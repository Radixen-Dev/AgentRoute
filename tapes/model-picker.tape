# See dashboard.tape's header comment — same rendering caveats apply.
#
# Navigates Dashboard -> Role Mapper -> Model Picker via the numbered jump
# keys, to show the picker's layout. demo.yml's runner has no OpenRouter
# key configured, so the picker is expected to show its empty/error state
# rather than a populated catalog — that's a known limitation of running
# this in CI without credentials, not a bug in the tape.

Output docs/demo/model-picker.gif

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

Type "3"
Sleep 1s

Type "4"
Sleep 2s

Escape
Sleep 1s

Type "q"
Sleep 500ms
