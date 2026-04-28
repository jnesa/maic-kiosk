-- newMasterCheckin SQLite schema (v2).
-- Applied verbatim by store.Open() the first time the DB is created.
-- Subsequent boots skip this file; alter via migrations/*.sql instead.

PRAGMA foreign_keys = ON;
PRAGMA journal_mode = WAL;

CREATE TABLE IF NOT EXISTS schema_version (
  version INTEGER NOT NULL PRIMARY KEY,
  applied_at TEXT NOT NULL
);

-- Internal MAIC operators only. Provisioned via the admin-cli; no public
-- self-registration. Disabled rows have status='disabled' and can't log in.
CREATE TABLE IF NOT EXISTS admin_users (
  id            INTEGER PRIMARY KEY,
  email         TEXT NOT NULL UNIQUE COLLATE NOCASE,
  name          TEXT NOT NULL,
  password_hash TEXT NOT NULL,
  status        TEXT NOT NULL DEFAULT 'active',
  created_at    TEXT NOT NULL,
  last_login_at TEXT
);

-- Cookie-backed sessions. Token = 256-bit random hex. Expiry slides on
-- every authed request (see internal/auth).
CREATE TABLE IF NOT EXISTS admin_sessions (
  token         TEXT PRIMARY KEY,
  admin_user_id INTEGER NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
  created_at    TEXT NOT NULL,
  expires_at    TEXT NOT NULL,
  user_agent    TEXT
);
CREATE INDEX IF NOT EXISTS admin_sessions_by_user ON admin_sessions(admin_user_id);

-- One row per legacy PMSApi subdomain.
CREATE TABLE IF NOT EXISTS hotels (
  id          INTEGER PRIMARY KEY,
  name        TEXT NOT NULL,
  pmsapi_url  TEXT NOT NULL,
  notes       TEXT,
  created_at  TEXT NOT NULL,
  updated_at  TEXT NOT NULL
);

-- One row per check-in URL. legacy_group_id is null when the hotel is a
-- single-tenant subdomain that doesn't use g_group.
CREATE TABLE IF NOT EXISTS kiosks (
  id                 TEXT PRIMARY KEY,
  hotel_id           INTEGER NOT NULL REFERENCES hotels(id) ON DELETE CASCADE,
  display_name       TEXT NOT NULL,
  legacy_group_id    INTEGER,
  legacy_group_label TEXT,
  theme              TEXT NOT NULL,
  languages          TEXT NOT NULL,
  device_key         TEXT NOT NULL,
  hero_image         TEXT,
  logo               TEXT,
  support_phone      TEXT,
  support_email      TEXT,
  status             TEXT NOT NULL DEFAULT 'active',
  created_at         TEXT NOT NULL,
  updated_at         TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS kiosks_by_hotel ON kiosks(hotel_id);
CREATE INDEX IF NOT EXISTS kiosks_by_status ON kiosks(status);

-- Append-only audit trail of admin write actions. Secrets (device keys,
-- password hashes) are stripped before insert in handler code.
CREATE TABLE IF NOT EXISTS audit_log (
  id            INTEGER PRIMARY KEY,
  admin_user_id INTEGER REFERENCES admin_users(id) ON DELETE SET NULL,
  actor_email   TEXT,
  action        TEXT NOT NULL,
  entity_type   TEXT NOT NULL,
  entity_id     TEXT NOT NULL,
  payload       TEXT,
  created_at    TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS audit_log_recent ON audit_log(created_at DESC);

INSERT OR IGNORE INTO schema_version (version, applied_at)
VALUES (1, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'));
