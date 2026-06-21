// SPDX-License-Identifier: GPL-3.0-only

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/Radixen-Dev/AgentRoute/internal/fsutil"
	"github.com/Radixen-Dev/AgentRoute/internal/paths"
)

// gatewayState is written by `up` once the gateway and sidecar are both
// healthy and the platform is linked, and removed by `up`'s own clean
// shutdown handler. It exists only for the lifetime of a single `up`
// invocation, so `status` (and a standalone `link`/`unlink` run from
// another terminal) can discover the running gateway. AgentRoute is
// foreground-only (see the architecture decision in the Phase 7 design
// notes): there is no separate daemon process to restart from this file
// after a crash, only diagnostic state to read.
type gatewayState struct {
	Port        int       `json:"port"`
	Token       string    `json:"token"`
	Profile     string    `json:"profile"`
	SidecarPort int       `json:"sidecarPort"`
	StartedAt   time.Time `json:"startedAt"`
}

func writeGatewayState(st gatewayState) error {
	path, err := paths.GatewayStateFile()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("encode gateway state: %w", err)
	}
	return fsutil.AtomicWrite(path, append(data, '\n'), 0o600)
}

// readGatewayState returns ok=false (not an error) when no `up` is
// currently recorded as running.
func readGatewayState() (gatewayState, bool, error) {
	path, err := paths.GatewayStateFile()
	if err != nil {
		return gatewayState{}, false, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return gatewayState{}, false, nil
	}
	if err != nil {
		return gatewayState{}, false, fmt.Errorf("read %s: %w", path, err)
	}
	var st gatewayState
	if err := json.Unmarshal(data, &st); err != nil {
		return gatewayState{}, false, fmt.Errorf("parse %s: %w", path, err)
	}
	return st, true, nil
}

func removeGatewayState() error {
	path, err := paths.GatewayStateFile()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	return nil
}
