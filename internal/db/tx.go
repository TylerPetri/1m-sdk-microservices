package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// WithTx runs fn inside a database transaction.
//
// Rules:
//   - fn must not call Commit/Rollback.
//   - if fn returns an error, the tx is rolled back.
//   - commit errors are returned.
func WithTx(ctx context.Context, pool *pgxpool.Pool, opts pgx.TxOptions, fn func(ctx context.Context, tx pgx.Tx) error) (err error) {
	if ctx == nil {
		return errors.New("db: nil context")
	}
	if pool == nil {
		return errors.New("db: nil pool")
	}
	if fn == nil {
		return errors.New("db: nil fn")
	}

	tx, err := pool.BeginTx(ctx, opts)
	if err != nil {
		return fmt.Errorf("db: begin tx: %w", err)
	}
	defer func() {
		// If fn returned error or a panic happened, roll back.
		// If commit succeeded, rollback will return ErrTxClosed and we ignore it.
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	if err = fn(ctx, tx); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("db: commit tx: %w", err)
	}
	return nil
}

// WithSerializableTx is a convenience wrapper around WithTx using SERIALIZABLE isolation.
func WithSerializableTx(ctx context.Context, pool *pgxpool.Pool, fn func(ctx context.Context, tx pgx.Tx) error) error {
	return WithTx(ctx, pool, pgx.TxOptions{IsoLevel: pgx.Serializable}, fn)
}

// WithRepeatableReadTx is a convenience wrapper around WithTx using REPEATABLE READ isolation.
func WithRepeatableReadTx(ctx context.Context, pool *pgxpool.Pool, fn func(ctx context.Context, tx pgx.Tx) error) error {
	return WithTx(ctx, pool, pgx.TxOptions{IsoLevel: pgx.RepeatableRead}, fn)
}
