package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Options struct {
	// Pool sizing
	MaxConns int32
	MinConns int32

	// Lifetime/idle tuning
	MaxConnLifetime time.Duration
	MaxConnIdleTime time.Duration

	// Background health checks
	HealthCheckPeriod time.Duration

	// Startup readiness check
	InitialPingTimeout time.Duration
}

func (o Options) withDefaults() Options {
	if o.InitialPingTimeout <= 0 {
		o.InitialPingTimeout = 2 * time.Second
	}
	// Leave other fields as zero-by-default meaning "pgx default"
	// unless you want explicit defaults.
	return o
}

func NewPool(ctx context.Context, dsn string, opts Options) (*pgxpool.Pool, error) {
	if ctx == nil {
		return nil, errors.New("db: nil context")
	}
	if dsn == "" {
		return nil, errors.New("db: empty DSN")
	}

	opts = opts.withDefaults()

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("db: parse config: %w", err)
	}

	// Apply pool tuning if set (zero means "keep pgx defaults").
	if opts.MaxConns > 0 {
		cfg.MaxConns = opts.MaxConns
	}
	if opts.MinConns > 0 {
		cfg.MinConns = opts.MinConns
	}

	// Guardrail: min should not exceed max (when both set).
	if cfg.MinConns > 0 && cfg.MaxConns > 0 && cfg.MinConns > cfg.MaxConns {
		return nil, fmt.Errorf("db: invalid pool sizing: MinConns(%d) > MaxConns(%d)", cfg.MinConns, cfg.MaxConns)
	}

	if opts.MaxConnLifetime > 0 {
		cfg.MaxConnLifetime = opts.MaxConnLifetime
	}
	if opts.MaxConnIdleTime > 0 {
		cfg.MaxConnIdleTime = opts.MaxConnIdleTime
	}
	if opts.HealthCheckPeriod > 0 {
		cfg.HealthCheckPeriod = opts.HealthCheckPeriod
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("db: create pool: %w", err)
	}

	// Fail fast on bad DSN / network / auth with a short ping timeout.
	pingCtx, cancel := context.WithTimeout(ctx, opts.InitialPingTimeout)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: initial ping: %w", err)
	}

	return pool, nil
}
