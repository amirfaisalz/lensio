-- Server-side session revocation.
--
-- Session tokens are stateless HS256 JWTs, so logout previously only cleared the
-- cookie: a token already copied out of the browser stayed valid until it expired.
-- sessions_valid_from records the instant all older sessions stop being accepted;
-- the validator rejects any token issued before it.
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS sessions_valid_from TIMESTAMPTZ NOT NULL DEFAULT 'epoch';
