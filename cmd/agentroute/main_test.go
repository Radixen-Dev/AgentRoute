// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// buildAgentRoute compiles the real binary once per test run, so these
// tests exercise main.go's actual error -> os.Exit(code) wiring — not
// just the errors.As(err, &exitCoder) logic in isolation (that's already
// covered by internal/cli's own tests; this test exists to catch a
// refactor that breaks the chain between the two).
func buildAgentRoute(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "agentroute-test-bin")
	if os.PathSeparator == '\\' {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin, ".")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return bin
}

func TestExitCodesEndToEnd(t *testing.T) {
	bin := buildAgentRoute(t)

	dir := t.TempDir()
	env := append(os.Environ(),
		"APPDATA="+dir,
		"XDG_CONFIG_HOME="+dir,
	)

	cases := []struct {
		name string
		args []string
		env  []string
		want int
	}{
		{name: "version succeeds", args: []string{"version"}, want: 0},
		{name: "unknown platform is usage error", args: []string{"unlink", "ghost-tool"}, want: 2},
		{name: "missing profile is usage error", args: []string{"profiles", "activate", "ghost"}, want: 2},
		{
			name: "no key configured for models",
			args: []string{"models"},
			env:  append(append([]string{}, env...), "OPENROUTER_API_KEY="),
			want: 3,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cmd := exec.Command(bin, c.args...)
			cmd.Env = env
			if c.env != nil {
				cmd.Env = c.env
			}
			_ = cmd.Run()

			got := cmd.ProcessState.ExitCode()
			if got != c.want {
				t.Fatalf("got exit code %d, want %d", got, c.want)
			}
		})
	}
}
