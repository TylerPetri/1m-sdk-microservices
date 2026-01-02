package boot

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"sdk-microservices/internal/platform/admin"
	"sdk-microservices/internal/platform/config"
	"sdk-microservices/internal/platform/health"
	"sdk-microservices/internal/platform/logging"
	"sdk-microservices/internal/platform/otel"

	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

// Main represents the primary server (HTTP or gRPC) for a service.
type Main struct {
	Serve    func() error
	Shutdown func(context.Context) error
}

// Deps are the shared platform dependencies provided to each service.
type Deps struct {
	Log       *zap.Logger
	Metrics   http.Handler
	ReadyRoot *health.Node
	Serving   *atomic.Bool
}

// Options configures the platform boot.
type Options struct {
	ServiceName string

	// AdminAddrEnv is the env var for the admin listener (defaults to <SERVICE>_ADMIN_ADDR).
	// AdminAddrFallback is used if env var is empty (defaults to :8081).
	AdminAddrEnv      string
	AdminAddrFallback string

	// OTELExtraAttrs are added to both tracing + metrics resources.
	OTELExtraAttrs []attribute.KeyValue

	// ShutdownTimeout bounds graceful shutdown.
	ShutdownTimeout time.Duration
}

// Run boots common platform pieces (logger, OTEL, metrics, admin server, readiness root),
// then runs the service's main server and blocks until it exits or a shutdown signal arrives.
func Run(ctx context.Context, opts Options, build func(ctx context.Context, deps Deps) (Main, error)) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if opts.ServiceName == "" {
		return errors.New("boot: ServiceName is required")
	}
	if opts.ShutdownTimeout <= 0 {
		opts.ShutdownTimeout = 10 * time.Second
	}

	log, err := logging.New(opts.ServiceName)
	if err != nil {
		return err
	}
	defer func() { _ = log.Sync() }()

	// Root context is canceled on SIGINT/SIGTERM or when main server errors.
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigc := make(chan os.Signal, 2)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigc)

	// OTEL tracing + metrics.
	shutdownTrace, err := otel.Init(runCtx, opts.ServiceName, opts.OTELExtraAttrs...)
	if err != nil {
		return err
	}
	metricsH, shutdownMetrics, err := otel.InitMetricsPrometheus(runCtx, opts.ServiceName, opts.OTELExtraAttrs...)
	if err != nil {
		_ = shutdownTrace(context.Background())
		return err
	}

	// Readiness graph (admin exposes /readyz using this root).
	ready := health.NewReadyGraph()
	ready.Add("otel", health.CheckAlwaysReady())
	ready.Add("metrics", health.CheckAlwaysReady())

	var serving atomic.Bool
	serving.Store(true)

	deps := Deps{
		Log:       log,
		Metrics:   metricsH,
		ReadyRoot: ready,
		Serving:   &serving,
	}

	// Admin server.
	adminEnv := opts.AdminAddrEnv
	if adminEnv == "" {
		adminEnv = upperServiceEnvPrefix(opts.ServiceName) + "_ADMIN_ADDR"
	}
	adminAddr := config.Getenv(adminEnv, ":8081")
	if opts.AdminAddrFallback != "" {
		adminAddr = config.Getenv(adminEnv, opts.AdminAddrFallback)
	}

	adminSrv, err := admin.Start(log, admin.Options{
		Addr:        adminAddr,
		ServiceName: opts.ServiceName,
		Metrics:     metricsH,
		ReadyRoot:   ready,
		ServingFn:   serving.Load,
	})
	if err != nil {
		_ = shutdownMetrics(context.Background())
		_ = shutdownTrace(context.Background())
		return err
	}
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), opts.ShutdownTimeout)
		defer shutdownCancel()
		_ = adminSrv.Shutdown(shutdownCtx)
	}()

	main, err := build(runCtx, deps)
	if err != nil {
		return err
	}
	if main.Serve == nil || main.Shutdown == nil {
		return errors.New("boot: Main.Serve and Main.Shutdown are required")
	}

	errCh := make(chan error, 1)
	go func() { errCh <- main.Serve() }()

	select {
	case <-runCtx.Done():
		// parent canceled
	case sig := <-sigc:
		log.Info("shutdown signal", zap.String("signal", sig.String()))
		cancel()
	case err := <-errCh:
		if err != nil {
			log.Error("main server exited", zap.Error(err))
		}
		cancel()
	}

	// Stop advertising readiness before shutdown.
	serving.Store(false)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), opts.ShutdownTimeout)
	defer shutdownCancel()

	var errs []error
	if err := main.Shutdown(shutdownCtx); err != nil {
		errs = append(errs, err)
	}
	if err := adminSrv.Shutdown(shutdownCtx); err != nil {
		errs = append(errs, err)
	}
	if err := shutdownMetrics(shutdownCtx); err != nil {
		errs = append(errs, err)
	}
	if err := shutdownTrace(shutdownCtx); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func upperServiceEnvPrefix(service string) string {
	// "gateway" -> "GATEWAY", "authd" -> "AUTH" (strip trailing d), etc.
	// Be conservative: uppercase and replace '-' with '_'.
	s := service
	if len(s) > 1 && s[len(s)-1] == 'd' {
		s = s[:len(s)-1]
	}
	b := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z':
			b = append(b, c-('a'-'A'))
		case c >= 'A' && c <= 'Z':
			b = append(b, c)
		case c == '-' || c == ' ':
			b = append(b, '_')
		default:
			b = append(b, c)
		}
	}
	return string(b)
}
