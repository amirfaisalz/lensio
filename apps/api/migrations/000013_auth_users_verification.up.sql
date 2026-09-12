-- Migration: 000013_auth_users_verification.up.sql
-- Allow users to register without an organization initially
ALTER TABLE users ALTER COLUMN org_id DROP NOT NULL;

-- Authentication & email verification columns
ALTER TABLE users ADD COLUMN IF NOT EXISTS password_hash VARCHAR(255);
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verified BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS verification_token VARCHAR(255);

-- Index for verification token lookup
CREATE INDEX IF NOT EXISTS idx_users_verification_token ON users (verification_token);

-- Pre-verify seed developer user
UPDATE users 
SET email_verified = TRUE 
WHERE email = 'dev@lensio.dev';
