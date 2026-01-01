package store

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound       = errors.New("not found")
	ErrRefreshInvalid = errors.New("refresh token invalid")
	ErrRefreshRevoked = errors.New("refresh token revoked")
	ErrRefreshExpired = errors.New("refresh token expired")
)

type Store struct {
	DB *pgxpool.Pool
}

type User struct {
	ID              string
	Email           string
	PasswordHash    string
	EmailVerifiedAt *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type SessionMeta struct {
	UserAgent string
	IP        net.IP
}

type Session struct {
	ID          string
	UserID      string
	ExpiresAt   time.Time
	RevokedAt   *time.Time
	CreatedAt   time.Time
	UserAgent   *string
	IP          net.IP
	RotatedFrom *string
}

func New(db *pgxpool.Pool) *Store { return &Store{DB: db} }

// --- Users ---

func (s *Store) CreateUser(ctx context.Context, email, passwordHash string) (*User, error) {
	var u User
	err := s.DB.QueryRow(ctx, `
        INSERT INTO users (email, password_hash)
        VALUES ($1, $2)
        RETURNING id::text, email, password_hash, email_verified_at, created_at, updated_at
    `, email, passwordHash).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.EmailVerifiedAt, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	var u User
	err := s.DB.QueryRow(ctx, `
        SELECT id::text, email, password_hash, email_verified_at, created_at, updated_at
        FROM users
        WHERE email = $1
    `, email).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.EmailVerifiedAt, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

func (s *Store) GetUserByID(ctx context.Context, id string) (*User, error) {
	var u User
	err := s.DB.QueryRow(ctx, `
        SELECT id::text, email, password_hash, email_verified_at, created_at, updated_at
        FROM users
        WHERE id = $1::uuid
    `, id).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.EmailVerifiedAt, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

// --- Sessions / refresh tokens ---

// HashRefreshToken returns the DB-stored hash (sha256) for an opaque refresh token.
// This is duplicated here to keep store self-contained for tests.
func HashRefreshToken(tok string) []byte {
	h := sha256.Sum256([]byte(tok))
	return h[:]
}

func (s *Store) CreateSession(ctx context.Context, userID string, refreshToken string, expiresAt time.Time, meta SessionMeta, rotatedFrom *string) (string, error) {
	hash := HashRefreshToken(refreshToken)

	var ip any
	if meta.IP != nil {
		ip = meta.IP.String()
	}
	var rotatedFromUUID any
	if rotatedFrom != nil && *rotatedFrom != "" {
		rotatedFromUUID = *rotatedFrom
	}

	var id string
	err := s.DB.QueryRow(ctx, `
        INSERT INTO sessions (user_id, refresh_token_hash, expires_at, user_agent, ip, rotated_from)
        VALUES ($1::uuid, $2, $3, NULLIF($4, ''), $5::inet, $6::uuid)
        RETURNING id::text
    `, userID, hash, expiresAt, meta.UserAgent, ip, rotatedFromUUID).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}

// ValidateRefresh validates a refresh token, returning the locked session row and user.
// This method does NOT rotate; use RotateRefresh for the transactional rotation flow.
func (s *Store) ValidateRefresh(ctx context.Context, refreshToken string) (*Session, *User, error) {
	hash := HashRefreshToken(refreshToken)

	var sess Session
	err := s.DB.QueryRow(ctx, `
        SELECT id::text, user_id::text, created_at, expires_at, revoked_at, user_agent, ip::text, rotated_from::text
        FROM sessions
        WHERE refresh_token_hash = $1
          AND revoked_at IS NULL
          AND expires_at > now()
    `, hash).Scan(&sess.ID, &sess.UserID, &sess.CreatedAt, &sess.ExpiresAt, &sess.RevokedAt, &sess.UserAgent, &ipTextOrNull{&sess.IP}, &sess.RotatedFrom)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, ErrRefreshInvalid
		}
		return nil, nil, err
	}

	u, err := s.GetUserByID(ctx, sess.UserID)
	if err != nil {
		// Don't leak existence. Treat as invalid.
		return nil, nil, ErrRefreshInvalid
	}
	return &sess, u, nil
}

// RotateRefresh atomically rotates a refresh token:
//  1. lock the old session row (FOR UPDATE)
//  2. verify not revoked/expired
//  3. revoke old
//  4. create new
//
// This prevents double-rotation races under concurrency.
func (s *Store) RotateRefresh(ctx context.Context, oldRefreshToken string, newRefreshToken string, newExpiresAt time.Time, meta SessionMeta) (newSessionID string, user *User, err error) {
	oldHash := HashRefreshToken(oldRefreshToken)
	newHash := HashRefreshToken(newRefreshToken)

	tx, err := s.DB.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return "", nil, err
	}
	defer func() {
		// Best-effort rollback; commit clears it.
		_ = tx.Rollback(ctx)
	}()

	var oldSessionID string
	var userID string
	var expiresAt time.Time
	var revokedAt *time.Time
	if err := tx.QueryRow(ctx, `
        SELECT id::text, user_id::text, expires_at, revoked_at
        FROM sessions
        WHERE refresh_token_hash = $1
        FOR UPDATE
    `, oldHash).Scan(&oldSessionID, &userID, &expiresAt, &revokedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil, ErrRefreshInvalid
		}
		return "", nil, err
	}

	now := time.Now().UTC()
	if revokedAt != nil {
		return "", nil, ErrRefreshRevoked
	}
	if !expiresAt.After(now) {
		return "", nil, ErrRefreshExpired
	}

	// Revoke old (idempotent under lock).
	if _, err := tx.Exec(ctx, `
        UPDATE sessions
        SET revoked_at = $2
        WHERE id = $1::uuid AND revoked_at IS NULL
    `, oldSessionID, now); err != nil {
		return "", nil, err
	}

	var ip any
	if meta.IP != nil {
		ip = meta.IP.String()
	}

	if err := tx.QueryRow(ctx, `
        INSERT INTO sessions (user_id, refresh_token_hash, expires_at, user_agent, ip, rotated_from)
        VALUES ($1::uuid, $2, $3, NULLIF($4, ''), $5::inet, $6::uuid)
        RETURNING id::text
    `, userID, newHash, newExpiresAt, meta.UserAgent, ip, oldSessionID).Scan(&newSessionID); err != nil {
		return "", nil, err
	}

	u, err := getUserByIDTx(ctx, tx, userID)
	if err != nil {
		return "", nil, ErrRefreshInvalid
	}

	if err := tx.Commit(ctx); err != nil {
		return "", nil, err
	}

	return newSessionID, u, nil
}

func (s *Store) RevokeSessionByRefresh(ctx context.Context, refreshToken string) error {
	hash := HashRefreshToken(refreshToken)
	ct, err := s.DB.Exec(ctx, `
        UPDATE sessions
        SET revoked_at = now()
        WHERE refresh_token_hash = $1
          AND revoked_at IS NULL
    `, hash)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrRefreshInvalid
	}
	return nil
}

func (s *Store) RevokeAll(ctx context.Context, userID string) error {
	_, err := s.DB.Exec(ctx, `
        UPDATE sessions
        SET revoked_at = now()
        WHERE user_id = $1::uuid AND revoked_at IS NULL
    `, userID)
	return err
}

// --- helpers ---

// ipTextOrNull scans a nullable text column into net.IP.
type ipTextOrNull struct{ dst *net.IP }

func (s *ipTextOrNull) Scan(src any) error {
	if s.dst == nil {
		return nil
	}
	switch v := src.(type) {
	case nil:
		*s.dst = nil
		return nil
	case string:
		if v == "" {
			*s.dst = nil
			return nil
		}
		ip := net.ParseIP(v)
		if ip == nil {
			return fmt.Errorf("invalid ip: %q", v)
		}
		*s.dst = ip
		return nil
	default:
		return fmt.Errorf("unsupported ip scan type %T", src)
	}
}

func getUserByIDTx(ctx context.Context, tx pgx.Tx, id string) (*User, error) {
	var u User
	err := tx.QueryRow(ctx, `
        SELECT id::text, email, password_hash, email_verified_at, created_at, updated_at
        FROM users
        WHERE id = $1::uuid
    `, id).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.EmailVerifiedAt, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}
