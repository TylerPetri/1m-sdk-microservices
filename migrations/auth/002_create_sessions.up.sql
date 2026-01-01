-- Sessions (refresh tokens)
--
-- Refresh tokens are opaque, random secrets that are stored *hashed* (sha256)
-- so a DB leak does not leak active refresh tokens.

CREATE TABLE IF NOT EXISTS sessions (
  id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id            UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  refresh_token_hash BYTEA NOT NULL UNIQUE,
  created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at         TIMESTAMPTZ NOT NULL,
  revoked_at         TIMESTAMPTZ NULL,
  -- future hooks
  user_agent         TEXT NULL,
  ip                 INET NULL,
  rotated_from       UUID NULL REFERENCES sessions(id)
);

CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);
-- Handy for cleanup / queries that only care about revoked sessions.
CREATE INDEX IF NOT EXISTS idx_sessions_revoked_at_not_null ON sessions(revoked_at) WHERE revoked_at IS NOT NULL;
