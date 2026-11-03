-- Initial schema for local account authentication.
-- Login design spec §3.1: users, password_history, audit_logs.
-- Applied by golang-migrate at apiserver startup, idempotent once applied
-- (schema_migrations tracks the version) and re-runnable from a clean schema.

CREATE TABLE users (
  id                  BIGSERIAL PRIMARY KEY,
  username            TEXT NOT NULL UNIQUE,
  password_hash       TEXT NOT NULL,              -- PHC: $argon2id$v=19$m=65536,t=3,p=4$...
  role                TEXT NOT NULL CHECK (role IN ('user','sysadmin','secadmin','auditadmin')),
  status              TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','active','disabled')),
  must_change_password BOOLEAN NOT NULL DEFAULT TRUE,
  created_by          BIGINT REFERENCES users(id),
  approved_by         BIGINT REFERENCES users(id),
  created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_login_at       TIMESTAMPTZ
);

CREATE TABLE audit_logs (
  id             BIGSERIAL PRIMARY KEY,
  actor_id       BIGINT,
  actor_username TEXT NOT NULL,
  actor_role     TEXT,
  action         TEXT NOT NULL,
  target         TEXT,
  detail         JSONB,
  ip             INET,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_audit_created ON audit_logs(created_at);
CREATE INDEX idx_audit_actor ON audit_logs(actor_id);

CREATE TABLE password_history (
  id            BIGSERIAL PRIMARY KEY,
  user_id       BIGINT NOT NULL REFERENCES users(id),
  password_hash TEXT NOT NULL,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_password_history_user ON password_history(user_id, created_at DESC);
