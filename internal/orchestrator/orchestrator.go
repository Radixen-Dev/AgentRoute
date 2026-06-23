// SPDX-License-Identifier: GPL-3.0-only

// Package orchestrator owns the gateway+sidecar+link lifecycle that both
// `agentroute up` (plain CLI) and the TUI's Dashboard/Gateway screen drive.
// It existed first as inline code in internal/cli/up.go; it was extracted
// here so the TUI can start/stop the exact same lifecycle in-process
// instead of shelling out to its own binary.
package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Radixen-Dev/AgentRoute/internal/config"
	"github.com/Radixen-Dev/AgentRoute/internal/fsutil"
	"github.com/Radixen-Dev/AgentRoute/internal/gateway"
	"github.com/Radixen-Dev/AgentRoute/internal/paths"
	"github.com/Radixen-Dev/AgentRoute/internal/platform"
	"github.com/Radixen-Dev/AgentRoute/internal/platform/claudecode"
	"github.com/Radixen-Dev/AgentRoute/internal/profile"
	"github.com/Radixen-Dev/AgentRoute/internal/secret"
	"github.com/Radixen-Dev/AgentRoute/internal/sidecar"
)

// Sentinel errors callers can match with errors.Is to decide a plain-mode
// exit code (see internal/cli/exitcode.go) or a TUI toast severity. Errors
// returned by Start that don't match any of these are gateway/sidecar
// failures (ExitGatewayFailed in plain mode).
var (
	ErrNoActiveProfile = errors.New("no profile specified and no active profile set")
	ErrEmptyProfile    = errors.New("profile has no tier models configured")
	ErrMissingAPIKey   = errors.New("no OpenRouter API key configured")
	ErrLinkFailed      = errors.New("platform link failed")
)

// Options configures a Start call.
type Options struct {
	ProfileName string // empty means "use the active profile"
	Port        int    // 0 means "use the configured default"
	NoLink      bool
}

// Deps is the dependency-injection seam: tests (and the TUI, eventually)
// substitute fakes here instead of a real litellm binary or a real
// ~/.claude/settings.json.
type Deps struct {
	NewSupervisor func() *sidecar.Supervisor
	NewAdapter    func() platform.Platform
}

// DefaultDeps wires the real sidecar supervisor and the real Claude Code
// adapter (~/.claude/settings.json).
func DefaultDeps() Deps {
	return Deps{
		NewSupervisor: func() *sidecar.Supervisor { return &sidecar.Supervisor{} },
		NewAdapter:    func() platform.Platform { return claudecode.New() },
	}
}

// Logf receives human-readable progress lines during Start (e.g. "starting
// LiteLLM sidecar on port 51234..."). May be nil.
type Logf func(format string, args ...any)

// Running is a started gateway+sidecar(+link), live until Stop is called or
// the gateway exits unexpectedly (watch Done()).
type Running struct {
	Server       *gateway.Server
	Supervisor   *sidecar.Supervisor
	Adapter      platform.Platform
	ProfileName  string
	Profile      profile.Profile
	SidecarPort  int
	GatewayToken string
	Linked       bool
	StartedAt    time.Time

	serveErrCh chan error
	cleanups   []func()
	mu         sync.Mutex
	stopped    bool
}

// Done reports the channel the gateway's Serve error (or nil, on a normal
// Shutdown) is delivered to. Callers select on this alongside their own
// cancellation to detect an unexpected gateway exit.
func (r *Running) Done() <-chan error { return r.serveErrCh }

// Stop tears down everything Start built, in reverse order: unlink (if
// linked) -> stop gateway -> stop sidecar -> remove rendered config. Safe
// to call more than once; only the first call does anything.
func (r *Running) Stop(_ context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stopped {
		return
	}
	r.stopped = true
	for i := len(r.cleanups) - 1; i >= 0; i-- {
		r.cleanups[i]()
	}
}

// Start renders the LiteLLM config, starts the sidecar, starts the
// gateway, and (unless opts.NoLink) links the platform — unwinding
// everything already started if any step fails. The returned Running's
// Stop must be called to shut everything back down.
func Start(ctx context.Context, opts Options, deps Deps, logf Logf) (*Running, error) {
	if logf == nil {
		logf = func(string, ...any) {}
	}

	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	profileName := opts.ProfileName
	if profileName == "" {
		profileName = cfg.ActiveProfile
	}
	if profileName == "" {
		return nil, ErrNoActiveProfile
	}

	prof, err := profile.Load(profileName)
	if err != nil {
		return nil, err
	}
	if len(prof.Models) == 0 {
		return nil, fmt.Errorf("profile %q: %w", profileName, ErrEmptyProfile)
	}

	apiKey, _, err := secret.OpenRouterAPIKey()
	if err != nil {
		return nil, err
	}
	if apiKey == "" {
		return nil, ErrMissingAPIKey
	}

	gatewayPort := cfg.Port
	if opts.Port != 0 {
		gatewayPort = opts.Port
	}

	r := &Running{ProfileName: profileName, Profile: prof, serveErrCh: make(chan error, 1), StartedAt: time.Now()}
	push := func(f func()) { r.cleanups = append(r.cleanups, f) }
	unwound := false
	unwind := func() {
		if unwound {
			return
		}
		unwound = true
		for i := len(r.cleanups) - 1; i >= 0; i-- {
			r.cleanups[i]()
		}
	}

	sidecarPort, err := pickFreePort()
	if err != nil {
		unwind()
		return nil, fmt.Errorf("pick sidecar port: %w", err)
	}
	r.SidecarPort = sidecarPort

	masterKey, err := gateway.GenerateToken()
	if err != nil {
		unwind()
		return nil, err
	}

	sidecarDir, err := paths.SidecarDir()
	if err != nil {
		unwind()
		return nil, err
	}
	configPath := filepath.Join(sidecarDir, "litellm.yaml")

	rendered, err := sidecar.RenderConfig(prof, apiKey, masterKey)
	if err != nil {
		unwind()
		return nil, err
	}
	if err := fsutil.AtomicWrite(configPath, rendered, 0o600); err != nil {
		unwind()
		return nil, err
	}
	push(func() { _ = os.Remove(configPath) })

	supervisor := deps.NewSupervisor()
	r.Supervisor = supervisor
	logf("starting LiteLLM sidecar on port %d...", sidecarPort)
	if err := supervisor.Start(ctx, configPath, sidecarPort); err != nil {
		unwind()
		return nil, fmt.Errorf("sidecar: %w", err)
	}
	push(func() { _ = supervisor.Stop(10 * time.Second) })

	gatewayToken, err := gateway.GenerateToken()
	if err != nil {
		unwind()
		return nil, err
	}
	r.GatewayToken = gatewayToken

	srv, err := gateway.New(gateway.Config{
		Port:   gatewayPort,
		Token:  gatewayToken,
		Router: gateway.MapRouter(prof.Aliases()),
		Translators: []gateway.Translator{
			&gateway.AnthropicLiteLLMTranslator{
				SidecarURL:   fmt.Sprintf("http://127.0.0.1:%d", sidecarPort),
				SidecarToken: masterKey,
			},
		},
	})
	if err != nil {
		unwind()
		return nil, fmt.Errorf("gateway: %w", err)
	}
	r.Server = srv

	go func() { r.serveErrCh <- srv.Serve() }()
	push(func() { _ = srv.Shutdown(10 * time.Second) })
	logf("gateway listening on 127.0.0.1:%d", srv.Port())

	adapter := deps.NewAdapter()
	r.Adapter = adapter
	if !opts.NoLink {
		res, err := adapter.Link(ctx, platform.LinkInput{
			GatewayURL:  fmt.Sprintf("http://127.0.0.1:%d", srv.Port()),
			AuthToken:   gatewayToken,
			RoleAliases: prof.RoleAliases(),
		})
		if err != nil {
			unwind()
			return nil, fmt.Errorf("%w: %s: %w", ErrLinkFailed, adapter.ID(), err)
		}
		r.Linked = true
		push(func() { _ = adapter.Unlink(context.Background()) })
		logf("%s linked (config: %s)", adapter.ID(), res.ConfigPath)
	}

	return r, nil
}

func pickFreePort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer func() { _ = ln.Close() }()
	return ln.Addr().(*net.TCPAddr).Port, nil
}
