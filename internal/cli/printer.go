// SPDX-License-Identifier: GPL-3.0-only

package cli

import (
	"encoding/json"
	"fmt"
	"io"
)

// printer is the single point every command writes output through, so the
// stdout-is-machine-output / stderr-is-human-text split (plan §7.5) can
// never be accidentally violated by a stray fmt.Println in a command file.
type printer struct {
	out  io.Writer // stdout: NDJSON when JSON is true, plain text otherwise
	errw io.Writer // stderr: always human-readable text/diagnostics
	json bool
}

// JSON encodes v as one compact JSON object followed by a newline (NDJSON)
// to stdout. Used only when the command was invoked with --json.
func (p *printer) JSON(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("encode JSON output: %w", err)
	}
	_, err = fmt.Fprintln(p.out, string(data))
	return err
}

// Line writes a plain human-readable line to stdout. Callers must only use
// this when p.json is false; commands check p.json before choosing which
// of JSON/Line to call.
func (p *printer) Line(format string, args ...any) {
	_, _ = fmt.Fprintf(p.out, format+"\n", args...)
}

// Errf writes a human-readable diagnostic line to stderr. Always goes to
// stderr regardless of --json, per the plain-mode contract: machine output
// is stdout-only.
func (p *printer) Errf(format string, args ...any) {
	_, _ = fmt.Fprintf(p.errw, format+"\n", args...)
}
