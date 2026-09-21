-- Password reset.
--
-- Without this there was no recovery path at all: a forgotten password locked
-- the account permanently, because the only other way in (email verification)
-- does not grant a session.
--
-- The token is stored as a SHA-256 hash, for the same reason API keys are: a
-- reset token is a full account takeover if the database is ever read.
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS password_reset_token_hash VARCHAR(64),
    ADD COLUMN IF NOT EXISTS password_reset_expires_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_users_password_reset_token_hash
    ON users (password_reset_token_hash)
    WHERE password_reset_token_hash IS NOT NULL;
