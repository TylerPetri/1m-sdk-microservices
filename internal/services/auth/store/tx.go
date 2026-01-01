package store

import (
	"context"

	"github.com/jackc/pgx/v5"
	"sdk-microservices/internal/db"
)

// WithTx runs fn inside a transaction using this store's pool.
//
// Prefer Serializable for flows where concurrent correctness matters
// (e.g. refresh rotation), otherwise use default isolation.
func (s *Store) WithTx(ctx context.Context, opts pgx.TxOptions, fn func(ctx context.Context, tx pgx.Tx) error) error {
	return db.WithTx(ctx, s.DB, opts, fn)
}
