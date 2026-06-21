// SPDX-License-Identifier: GPL-3.0-only

//go:build !windows

package sidecar

import (
	"fmt"
	"os/exec"
	"syscall"
)

// setProcAttrs puts the subprocess in its own process group so
// killProcessGroup can terminate it and every child it spawns (LiteLLM is
// Python and may fork workers) without affecting AgentRoute's own process
// group.
func setProcAttrs(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProcessGroup sends SIGTERM to the entire process group rooted at
// cmd's pid (the negative-pid convention for kill(2)).
func killProcessGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM); err != nil && err != syscall.ESRCH {
		return fmt.Errorf("kill process group %d: %w", cmd.Process.Pid, err)
	}
	return nil
}
