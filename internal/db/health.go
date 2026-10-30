package db

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Check(ctx context.Context, pool *pgxpool.Pool) error {
	if ctx == nil {
		return errors.New("db: nil context")
	}
	if pool == nil {
		return errors.New("db: nil pool")
	}

	// SELECT 1 is intentionally trivial:
	// - hits the wire
	// - validates auth + routing
	// - exercises a real connection
	var one int
	if err := pool.QueryRow(ctx, "select 1").Scan(&one); err != nil {
		return err
	}
	return nil
}
