-- name: CreateUser :one
INSERT INTO users (email, password_hash)
VALUES ($1, $2)
RETURNING id::text AS id, email, password_hash, email_verified_at;

-- name: GetUserByEmail :one
SELECT id::text AS id, email, password_hash, email_verified_at
FROM users
WHERE email = $1;

-- name: GetUserByID :one
SELECT id::text AS id, email, password_hash, email_verified_at
FROM users
WHERE id = $1::uuid;

-- name: MarkEmailVerified :exec
UPDATE users
SET email_verified_at = $2,
    updated_at = now()
WHERE id = $1::uuid;

-- name: CreateSession :exec
INSERT INTO sessions (user_id, token_hash, expires_at)
VALUES ($1::uuid, $2, $3);

-- name: GetSessionUserIDByTokenHash :one
SELECT user_id::text AS user_id
FROM sessions
WHERE token_hash = $1
  AND revoked_at IS NULL
  AND expires_at > $2;

-- name: RevokeSessionByTokenHash :exec
UPDATE sessions
SET revoked_at = $2
WHERE token_hash = $1
  AND revoked_at IS NULL;

-- name: CreateEmailVerification :exec
INSERT INTO email_verifications (user_id, token_hash, expires_at)
VALUES ($1::uuid, $2, $3);

-- name: ConsumeEmailVerification :one
UPDATE email_verifications
SET consumed_at = $2
WHERE token_hash = $1
  AND consumed_at IS NULL
  AND expires_at > $2
RETURNING user_id::text AS user_id;
