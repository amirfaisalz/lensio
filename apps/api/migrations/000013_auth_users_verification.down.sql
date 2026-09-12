-- Migration: 000013_auth_users_verification.down.sql
DROP INDEX IF EXISTS idx_users_verification_token;

ALTER TABLE users DROP COLUMN IF EXISTS verification_token;
ALTER TABLE users DROP COLUMN IF EXISTS email_verified;
ALTER TABLE users DROP COLUMN IF EXISTS password_hash;

-- Delete any users without an org_id before restoring NOT NULL constraint
DELETE FROM users WHERE org_id IS NULL;

-- Restore NOT NULL constraint on org_id (requires all users to have an org_id)
ALTER TABLE users ALTER COLUMN org_id SET NOT NULL;
