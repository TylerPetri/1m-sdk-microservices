package admin

import (
	"context"
	"net"
	"net/http"
	"time"

	"sdk-microservices/internal/platform/health"

	"go.uber.org/zap"
)

// Server is a small admin HTTP server exposing /metrics, /livez, /readyz.
type Server struct {
	http *http.Server
	ln   net.Listener
}

type Options struct {
	Addr         string
	ServiceName  string
	Metrics      http.Handler // optional
	ReadyRoot    *health.Node // optional
	ServingFn    func() bool  // optional (NOT_SERVING gate)
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

func Start(log *zap.Logger, opts Options) (*Server, error) {
	if log == nil {
		log = zap.NewNop()
	}
	mux := http.NewServeMux()
	mux.Handle("/livez", health.Livez())
	if opts.ReadyRoot != nil {
		mux.Handle("/readyz", health.Handler(opts.ReadyRoot, opts.ServingFn))
	}
	if opts.Metrics != nil {
		mux.Handle("/metrics", opts.Metrics)
	}

	srv := &http.Server{
		Addr:         opts.Addr,
		Handler:      mux,
		ReadTimeout:  orDur(opts.ReadTimeout, 5*time.Second),
		WriteTimeout: orDur(opts.WriteTimeout, 10*time.Second),
		IdleTimeout:  orDur(opts.IdleTimeout, 60*time.Second),
	}

	ln, err := net.Listen("tcp", opts.Addr)
	if err != nil {
		return nil, err
	}

	as := &Server{http: srv, ln: ln}
	go func() {
		log.Info("admin server listening", zap.String("addr", opts.Addr))
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Error("admin server error", zap.Error(err))
		}
	}()
	return as, nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s == nil || s.http == nil {
		return nil
	}
	return s.http.Shutdown(ctx)
}

func orDur(v, d time.Duration) time.Duration {
	if v <= 0 {
		return d
	}
	return v
}
