package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	DB *pgxpool.Pool
}

type User struct {
	ID           string    `db:"id"`
	Email        string    `db:"email"`
	PasswordHash string    `db:"password_hash"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
}

func New(db *pgxpool.Pool) *Store {
	return &Store{DB: db}
}

func (s *Store) CreateUser(ctx context.Context, email, passwordHash string) (*User, error) {
	var u User
	s.DB.QueryRow(ctx, `
		INSERT INTO users (email, password_hash)
		VALUES ($1, $2)
		RETURNING id, email, password_hash, created_at, updated_at
	`, email, passwordHash)

	return &u, nil
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	var u User
	s.DB.QueryRow(ctx, `
		SELECT id, email, password_hash, created_at, updated_at
		FROM users
		WHERE email = $1
	`, email)
	return &u, nil
}
