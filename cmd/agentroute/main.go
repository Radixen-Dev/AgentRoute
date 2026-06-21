// SPDX-License-Identifier: GPL-3.0-only

// Command agentroute is AgentRoute's entrypoint: it builds the cobra root
// command (internal/cli) and translates its error into a process exit code.
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/Radixen-Dev/AgentRoute/internal/cli"
)

// exitCoder is satisfied by internal/cli's unexported exitError type; it
// is matched structurally by errors.As, so it does not need to be the
// same named type the cli package defines.
type exitCoder interface{ ExitCode() int }

func main() {
	root := cli.New()
	err := root.Execute()
	if err == nil {
		os.Exit(cli.ExitOK)
	}

	fmt.Fprintln(os.Stderr, "Error:", err)

	var ec exitCoder
	if errors.As(err, &ec) {
		os.Exit(ec.ExitCode())
	}
	os.Exit(cli.ExitGeneric)
}
