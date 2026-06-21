// SPDX-License-Identifier: GPL-3.0-only

package gateway

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// routeFor returns the path each wire's translator is mounted at. A
// trailing "/*" lets chi match subpaths (e.g. Anthropic's
// /v1/messages/count_tokens alongside /v1/messages).
var routeFor = map[Wire]string{
	WireOpenAI:    "/v1/chat/completions",
	WireAnthropic: "/v1/messages*",
	WireGemini:    "/v1beta/*",
}

// Config configures a gateway Server.
type Config struct {
	// Host defaults to "127.0.0.1". The gateway must never bind a
	// non-loopback address.
	Host string
	// Port to listen on. If 0, an arbitrary free port is chosen.
	Port int
	// Token is the Bearer credential every request must present.
	Token string
	// Upstream is where translated requests are forwarded.
	Upstream Upstream
	// Router resolves AgentRoute tier aliases to upstream model ids.
	Router ModelRouter
	// Translators is the set of wire formats this gateway instance
	// serves. v1 registers OpenAINativeTranslator directly and the
	// Anthropic LiteLLM-backed translator (added in Phase 5).
	Translators []Translator
	// Log records proxied requests for the TUI's live log screen. If nil,
	// a private unbounded-until-capacity log is created internally.
	Log *RequestLog
}

// Server is AgentRoute's local routing gateway.
type Server struct {
	cfg      Config
	listener net.Listener
	http     *http.Server
}

const defaultLogCapacity = 500

// New builds a Server bound to a loopback listener. The actual port in use
// (after auto-pick, if cfg.Port was 0 or busy) is available via Server.Port
// once New returns successfully.
func New(cfg Config) (*Server, error) {
	if cfg.Host == "" {
		cfg.Host = "127.0.0.1"
	}
	if cfg.Log == nil {
		cfg.Log = NewRequestLog(defaultLogCapacity)
	}

	listener, err := bind(cfg.Host, cfg.Port)
	if err != nil {
		return nil, fmt.Errorf("gateway: bind listener: %w", err)
	}

	router := chi.NewRouter()
	router.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	authed := chi.NewRouter()
	authed.Use(RequireBearer(cfg.Token))
	for _, t := range cfg.Translators {
		path, ok := routeFor[t.Wire()]
		if !ok {
			return nil, fmt.Errorf("gateway: no route configured for wire %q", t.Wire())
		}
		authed.Handle(path, t.Handler(cfg.Upstream, cfg.Router, cfg.Log))
	}
	router.Mount("/", authed)

	return &Server{
		cfg:      cfg,
		listener: listener,
		http:     &http.Server{Handler: router},
	}, nil
}

// Port returns the actual TCP port the server is bound to.
func (s *Server) Port() int {
	return s.listener.Addr().(*net.TCPAddr).Port
}

// RequestLog returns the request log backing this server's live log.
func (s *Server) RequestLog() *RequestLog {
	return s.cfg.Log
}

// Serve blocks, accepting connections until Shutdown is called. It
// returns nil on a clean shutdown (mirrors http.Server.Serve semantics
// with ErrServerClosed suppressed).
func (s *Server) Serve() error {
	err := s.http.Serve(s.listener)
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

// Shutdown gracefully stops the server, waiting up to the given timeout
// for in-flight requests (notably long SSE streams) to finish.
func (s *Server) Shutdown(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return s.http.Shutdown(ctx)
}

// bind listens on host:port. If port is 0, or the requested port is
// already in use, it falls back to an OS-assigned free port.
func bind(host string, port int) (net.Listener, error) {
	addr := fmt.Sprintf("%s:%d", host, port)
	listener, err := net.Listen("tcp", addr)
	if err == nil {
		return listener, nil
	}
	if port == 0 {
		return nil, err
	}
	// Requested port was busy; fall back to any free port.
	return net.Listen("tcp", fmt.Sprintf("%s:0", host))
}
