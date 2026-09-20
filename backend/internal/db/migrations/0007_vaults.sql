-- Anonymous vault access: a second way to obtain a `users` row and session,
-- alongside the existing email/password path (kept intact and optional —
-- see internal/auth/service.go). A "vault" is not a new entity; it's a user
-- row created without an email, identified by a public vault_id instead,
-- and authenticated with a high-entropy recovery key (hashed exactly like
-- a password) instead of email+password. Every downstream table already
-- scopes by users.id, so no other schema changes are needed.

ALTER TABLE users ALTER COLUMN email DROP NOT NULL;
ALTER TABLE users ADD COLUMN vault_id TEXT UNIQUE;

CREATE INDEX idx_users_vault_id ON users (vault_id) WHERE vault_id IS NOT NULL;

-- Exactly one of (email, vault_id) must be set — a user is either an
-- email/password account or an anonymous vault, never neither, never both.
ALTER TABLE users ADD CONSTRAINT users_email_xor_vault_id
    CHECK ((email IS NOT NULL) <> (vault_id IS NOT NULL));
