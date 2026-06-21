// SPDX-License-Identifier: GPL-3.0-only

//go:build windows

package sidecar

import (
	"fmt"
	"os/exec"
	"strconv"
	"syscall"
)

// setProcAttrs starts the subprocess in its own process group (so a
// Ctrl+C delivered to AgentRoute's own console does not also reach the
// sidecar; we terminate it explicitly via killProcessGroup instead).
func setProcAttrs(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}

// killProcessGroup terminates cmd's process and every descendant it spawned
// (LiteLLM is Python and may fork workers). Windows has no kill(-pid)
// equivalent; "taskkill /T" walks the process tree by parent pid, which is
// the standard way to reliably stop a process tree on Windows.
func killProcessGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	pid := strconv.Itoa(cmd.Process.Pid)
	out, err := exec.Command("taskkill", "/T", "/F", "/PID", pid).CombinedOutput()
	if err != nil {
		// taskkill exits non-zero if the process already exited; that is
		// not a failure for our purposes.
		if len(out) > 0 {
			return fmt.Errorf("taskkill: %w: %s", err, out)
		}
		return nil
	}
	return nil
}
