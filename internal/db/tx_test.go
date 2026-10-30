package db

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestWithTx_NilGuards(t *testing.T) {
	ctx := context.Background()
	if err := WithTx(nil, nil, pgx.TxOptions{}, func(context.Context, pgx.Tx) error { return nil }); err == nil {
		t.Fatalf("expected error for nil ctx")
	}
	if err := WithTx(ctx, nil, pgx.TxOptions{}, func(context.Context, pgx.Tx) error { return nil }); err == nil {
		t.Fatalf("expected error for nil pool")
	}
	// fn=nil should error even if pool is nil; we just ensure the guard is present.
	if err := WithTx(ctx, nil, pgx.TxOptions{}, nil); err == nil {
		t.Fatalf("expected error for nil fn")
	}
}
