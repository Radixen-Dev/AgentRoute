# See dashboard.tape's header comment — same rendering caveats apply.
#
# Demonstrates the plain CLI (AGENTROUTE_PLAIN=1, so this never tries to
# launch the TUI even though VHS gives it a real TTY). Deliberately stops
# short of a real `agentroute up`: demo.yml's runner has no OpenRouter key
# or litellm installed, so `up` would just fail with exit code 3 or 4 —
# not a useful thing to show. `doctor` demonstrates the same failure
# legibly instead, which is the point of the command.

Output docs/demo/up.gif

Set Shell bash
Set FontSize 16
Set Width 1200
Set Height 700
Set Theme "Dracula"
Set Padding 20
Set TypingSpeed 60ms

Type "export AGENTROUTE_PLAIN=1"
Enter
Sleep 500ms

Type "agentroute doctor"
Enter
Sleep 2s

Type "agentroute profiles list"
Enter
Sleep 1.5s

Type "agentroute up --help"
Enter
Sleep 2.5s
