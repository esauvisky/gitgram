// Package httpserver builds the HTTP listener that receives GitLab and
// Telegram webhooks and answers health checks.
package httpserver

import (
	"log/slog"
	"net/http"
	"time"
)

// Options configures New.
type Options struct {
	// Addr is the listen address, e.g. ":8080".
	Addr string
	// Health is called by GET /healthz; nil error → 200, otherwise 503.
	// A nil func always reports healthy.
	Health func() error
	// Logger receives one line per request. Nil uses slog.Default().
	Logger *slog.Logger
}

// Server is an *http.Server whose handler is a mux with /healthz
// pre-registered. Webhook handlers are mounted with Handle.
type Server struct {
	*http.Server
	mux *http.ServeMux
}

// New builds the server. Timeouts are sized for GitLab's 10 s webhook
// delivery deadline.
func New(opts Options) *Server {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if opts.Health != nil {
			if err := opts.Health(); err != nil {
				logger.Warn("healthz failed", "err", err)
				http.Error(w, err.Error(), http.StatusServiceUnavailable)
				return
			}
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte("ok\n"))
	})
	return &Server{
		Server: &http.Server{
			Addr:              opts.Addr,
			Handler:           requestLogger(logger, mux),
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       15 * time.Second,
			WriteTimeout:      15 * time.Second,
			IdleTimeout:       60 * time.Second,
			MaxHeaderBytes:    64 << 10,
		},
		mux: mux,
	}
}

// Handle mounts h at pattern on the server mux. The pattern follows
// net/http.ServeMux rules, so "POST /webhook/gitlab" and
// "/webhook/telegram/" (subtree) both work.
func (s *Server) Handle(pattern string, h http.Handler) {
	s.mux.Handle(pattern, h)
}
