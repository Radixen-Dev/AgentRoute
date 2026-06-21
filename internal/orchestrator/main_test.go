// SPDX-License-Identifier: GPL-3.0-only

package orchestrator

import (
	"flag"
	"net/http"
	"os"
	"testing"
)

// TestMain re-execs this test binary as a fake LiteLLM server when
// AGENTROUTE_FAKE_LITELLM=1 is set — the same helper-process pattern used
// by internal/sidecar and internal/cli (each package's test binary needs
// its own copy), so Start/Stop can be exercised without a real litellm
// install.
func TestMain(m *testing.M) {
	if os.Getenv("AGENTROUTE_FAKE_LITELLM") == "1" {
		runFakeLiteLLM()
		return
	}
	os.Exit(m.Run())
}

func runFakeLiteLLM() {
	fs := flag.NewFlagSet("litellm", flag.ContinueOnError)
	port := fs.String("port", "", "")
	_ = fs.String("config", "", "")
	if err := fs.Parse(os.Args[1:]); err != nil {
		os.Exit(2)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health/liveliness", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	srv := &http.Server{Addr: "127.0.0.1:" + *port, Handler: mux}
	if err := srv.ListenAndServe(); err != nil {
		os.Exit(1)
	}
}
