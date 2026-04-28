# Plan — Admin-managed multi-tenant kiosk (v2)

## Goal

Replace the YAML-only registry with a small internal admin panel where MAIC
operators can:

1. Register a hotel (its legacy PMSApi subdomain).
2. Create one or more kiosk links per hotel — one per `g_group`, or one
   for the whole subdomain if it doesn't use groups.
3. Get a copy-pasteable kiosk URL (opaque UUID) + QR code per kiosk.
4. Disable / rotate device keys / delete kiosks without redeploying.

Hosted on its own domain (`checkin.maiccube.com`). The admin panel lives
at `/admin/...`; the kiosk SPA lives at `/<kiosk-uuid>/...`. Same Go
process serves both.

The legacy PMSApi is unchanged from where we left off — still uses the
`/api/kiosk/*` patch we wrote, no migrations. New legacy work in this
iteration: zero (operator types `group_id` manually per the answers).

---

## Decisions (confirmed)

| Question | Decision |
| --- | --- |
| Admin auth | Per-user accounts in our DB. Email + password, bcrypt-hashed. No public registration. |
| Group discovery | Manual entry. Operator looks the group up on the legacy side and types `group_id` + `name`. |
| Kiosk URL scheme | Opaque UUID, e.g. `checkin.maiccube.com/k_8a3f9c1bf04a4d2faa8b7e1c2d3e4f01`. |
| Iteration scope | Plan + future-state docs + full MVP build in one go. |

---

## Architecture (target)

```
                                       checkin.maiccube.com
                                       ┌──────────────────────────────────────┐
                                       │  nginx                               │
                                       │   /admin/*  → admin SPA              │
                                       │   /api/admin/*  → Go (admin routes)  │
                                       │   /api/kiosk/*  → Go (kiosk routes)  │
                                       │   /<uuid>/*  → kiosk SPA             │
                                       │                                      │
                                       │  ┌──────────────────────────────┐    │
                                       │  │  Go service (single binary)  │    │
                                       │  │                              │    │
                                       │  │   • admin/             auth, hotels, kiosks, audit
                                       │  │   • kiosk/             /api/kiosk/v1 proxy (existing)
                                       │  │   • registry/db        SQLite — replaces properties.yaml
                                       │  └────────────┬─────────────┘    │
                                       │               │                       │
                                       │      data.db (SQLite)                 │
                                       └───────────────┼───────────────────────┘
                                                       │ X-Device-Key (per kiosk)
                                                       ▼
                                              ┌────────────────────────┐
                                              │  Legacy PMSApi         │
                                              │  pms.maiccube.com/...  │
                                              │  /api/kiosk/*          │
                                              └────────────────────────┘
```

The kiosk SPA + Go proxy code we already shipped stays. The only runtime
change is the **registry** — instead of reading `properties.yaml` at
boot, the proxy looks each kiosk up in SQLite by its UUID at request
time (with a small in-memory cache).

---

## Data model (SQLite)

```sql
-- Internal MAIC operators only. No public registration.
CREATE TABLE admin_users (
  id            INTEGER PRIMARY KEY,
  email         TEXT NOT NULL UNIQUE,
  name          TEXT NOT NULL,
  password_hash TEXT NOT NULL,         -- bcrypt
  created_at    TEXT NOT NULL,
  last_login_at TEXT
);

-- One row per legacy subdomain we connect to.
CREATE TABLE hotels (
  id          INTEGER PRIMARY KEY,
  name        TEXT NOT NULL,           -- "smart moov", "Schladming Group"
  pmsapi_url  TEXT NOT NULL,           -- "https://pms.maiccube.com" or "https://schladming.maiccube.com"
  notes       TEXT,
  created_at  TEXT NOT NULL,
  updated_at  TEXT NOT NULL
);

-- One row per check-in URL.
-- For a single-tenant subdomain: legacy_group_id is null, one kiosk per hotel.
-- For a multi-group hotel: one row per group, legacy_group_id set.
CREATE TABLE kiosks (
  id              TEXT PRIMARY KEY,    -- "k_<32 hex>", used in the URL
  hotel_id        INTEGER NOT NULL REFERENCES hotels(id) ON DELETE CASCADE,
  display_name    TEXT NOT NULL,       -- "smart moov — lobby", "Schladming Hotel A"
  legacy_group_id INTEGER,             -- maps to g_group.id on legacy PMSApi (null if single-tenant)
  legacy_group_label TEXT,             -- for the dashboard, eg "Hotel A"
  theme           TEXT NOT NULL,       -- "smart-moov" | "pareus" | future
  languages       TEXT NOT NULL,       -- JSON array: '["en","de","it"]'
  device_key      TEXT NOT NULL,       -- hex, generated when the kiosk is created/rotated
  hero_image      TEXT,                -- optional URL/path
  logo            TEXT,                -- optional URL/path
  support_phone   TEXT,
  support_email   TEXT,
  status          TEXT NOT NULL DEFAULT 'active',  -- 'active' | 'disabled'
  created_at      TEXT NOT NULL,
  updated_at      TEXT NOT NULL
);
CREATE INDEX kiosks_by_hotel ON kiosks(hotel_id);

-- Audit trail of admin actions.
CREATE TABLE audit_log (
  id            INTEGER PRIMARY KEY,
  admin_user_id INTEGER REFERENCES admin_users(id),
  action        TEXT NOT NULL,         -- "create_hotel" | "create_kiosk" | "rotate_key" | "disable_kiosk" | …
  entity_type   TEXT NOT NULL,         -- "hotel" | "kiosk" | "admin_user"
  entity_id     TEXT NOT NULL,
  payload       TEXT,                  -- JSON, scrubbed of secrets
  created_at    TEXT NOT NULL
);
CREATE INDEX audit_log_recent ON audit_log(created_at DESC);

-- Sessions (cookie-backed) — keeps password hashes off the wire after login.
CREATE TABLE admin_sessions (
  token         TEXT PRIMARY KEY,      -- random opaque
  admin_user_id INTEGER NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
  created_at    TEXT NOT NULL,
  expires_at    TEXT NOT NULL,
  user_agent    TEXT
);
CREATE INDEX admin_sessions_by_user ON admin_sessions(admin_user_id);
```

Migration story: at first boot the service runs the embedded SQL above
if `data.db` doesn't exist, then optionally seeds from
`properties.yaml` for backwards compatibility (one-shot import).

---

## API surface (target)

### Public kiosk (unchanged from v1, just routed by UUID instead of slug)

| Method | Path                                       | Notes |
| ------ | ------------------------------------------ | --- |
| GET    | `/health`                                  | unchanged |
| GET    | `/api/kiosk/v1/ready`                      | unchanged |
| GET    | `/api/kiosk/v1/{uuid}/config`              | sanitised (no device key, no PMSApi URL) |
| POST   | `/api/kiosk/v1/{uuid}/{lookup,select,form,save-guest,save-firm,submit}` | proxies to the kiosk's hotel.pmsapi_url + adds X-Device-Key |

### Admin

All under `/api/admin/v1/`. All except `/auth/login` require a valid
session cookie.

| Method | Path                                       | Purpose |
| ------ | ------------------------------------------ | ------- |
| POST   | `/auth/login`                              | email + password → sets `kiosk_admin_session` cookie |
| POST   | `/auth/logout`                             | clears cookie + revokes session row |
| GET    | `/me`                                      | currently logged-in admin user |
| GET    | `/hotels`                                  | list all hotels with kiosk counts |
| POST   | `/hotels`                                  | create a hotel `{name, pmsapi_url, notes?}` |
| GET    | `/hotels/{id}`                             | hotel detail + its kiosks |
| PATCH  | `/hotels/{id}`                             | update name/url/notes |
| DELETE | `/hotels/{id}`                             | cascade-deletes kiosks (audit-logged) |
| POST   | `/hotels/{id}/kiosks`                      | create a kiosk (generates UUID + device key) |
| GET    | `/kiosks/{id}`                             | kiosk detail (admin-only fields incl. device key) |
| PATCH  | `/kiosks/{id}`                             | update editable fields |
| POST   | `/kiosks/{id}/rotate-key`                  | new device key, invalidates the old one |
| POST   | `/kiosks/{id}/disable`                     | sets status='disabled' (URL stops responding) |
| POST   | `/kiosks/{id}/enable`                      | re-enables |
| DELETE | `/kiosks/{id}`                             | hard delete |
| GET    | `/audit-log?limit=&before=`                | paginated recent actions |

### Admin user provisioning (out-of-band)

No self-signup. New operators are added via a CLI tool:

```bash
go-proxy admin add-user --email ilija@maiccube.com --name "Ilija Evic"
# Prompts for password, bcrypts it, inserts into admin_users.
```

This avoids leaking a public registration surface and keeps the admin
panel internal-only.

---

## URL scheme

| Path                                  | Served by                  |
| ------------------------------------- | -------------------------- |
| `/`                                   | Redirect to `/admin/login` |
| `/admin/...`                          | Admin SPA (built artefact) |
| `/api/admin/v1/*`                     | Go admin routes            |
| `/api/kiosk/v1/*`                     | Go kiosk proxy             |
| `/k_<uuid>` and `/k_<uuid>/...`       | Kiosk SPA (built artefact) |
| `/health`, `/api/kiosk/v1/ready`      | Go health endpoints        |

Reserved top-level paths the kiosk SPA must NOT treat as a slug:
`admin`, `api`, `assets`, `locales`, `health`. Already handled in
`src/api/client.ts:RESERVED_TOP_LEVEL` — extend with `admin`.

---

## Auth model

- **Login**: `POST /api/admin/v1/auth/login` body `{email, password}`.
  Bcrypt-compare. On success: insert a row in `admin_sessions` with a
  256-bit random token (32 bytes hex). Set `kiosk_admin_session` cookie
  with `HttpOnly`, `Secure`, `SameSite=Lax`, `Path=/admin` and
  `Path=/api/admin`. 12-hour expiry, sliding (refreshed on each authed
  call).
- **CSRF**: same-origin admin SPA + `SameSite=Lax` is sufficient for
  the actions we ship. We add a double-submit token on `DELETE` and
  `rotate-key` for defence in depth.
- **Sessions table** allows immediate revocation when a user logs out
  or an admin row is disabled.

Why not JWT: revocation needs a denylist anyway, sessions table is
simpler. SQLite write cost is negligible at admin volumes.

---

## Onboarding flow (operator perspective)

```
[Add Hotel]
  └─ name: "smart moov"
     pmsapi_url: https://pms.maiccube.com
     notes: "single tenant, group_id 72"
     [Save]
        ↓
[Hotel detail]
  └─ Kiosks: (empty)
     [+ Add kiosk]
        ↓
[Kiosk form]
     display_name: "smart moov — lobby tablet"
     legacy_group_id: 72        (operator types this from the legacy g_group table)
     legacy_group_label: "smart moov"
     theme: smart-moov
     languages: [en, de, it]
     hero_image: /themes/smart-moov/hero.jpg   (optional)
     [Generate device key & save]
        ↓
[Kiosk detail card]
  ┌────────────────────────────────────────────────────┐
  │ smart moov — lobby tablet           [Active]       │
  │                                                     │
  │ Kiosk URL                                           │
  │ https://checkin.maiccube.com/k_8a3f9c1bf04a4d2f...  │
  │ [Copy] [QR code]                                    │
  │                                                     │
  │ Legacy device key (paste into PMSApi .env)          │
  │ KIOSK_DEVICE_KEY=9d3feacffaf9be4e4d63f...           │
  │ KIOSK_GROUP_ID=72                                   │
  │ [Copy] [Rotate]                                     │
  │                                                     │
  │ Status: Active   [Disable]                          │
  └────────────────────────────────────────────────────┘
```

The "device key" the operator pastes into the legacy `.env` is the same
secret stored in `kiosks.device_key`. The operator copies it once, sets
it on PMSApi, never sees it again. Rotation regenerates both sides
(legacy `.env` update is still manual — that's fundamental, can't be
automated without legacy admin API access).

---

## Repo layout (target)

```
.
├── go-server/                     ← renamed from go-proxy (it's now more)
│   ├── cmd/
│   │   ├── server/main.go         — HTTP server + boot
│   │   └── admin/main.go          — CLI: add-user, list-users, reset-password
│   ├── internal/
│   │   ├── config/                — env, paths
│   │   ├── store/                 — SQLite repository (replaces YAML loader)
│   │   │   ├── schema.sql
│   │   │   ├── store.go           — Open(path), migrate
│   │   │   ├── hotels.go
│   │   │   ├── kiosks.go
│   │   │   ├── users.go
│   │   │   ├── sessions.go
│   │   │   └── audit.go
│   │   ├── auth/                  — bcrypt, sessions, middleware
│   │   ├── admin/                 — REST handlers for /api/admin/v1/*
│   │   ├── kiosk/                 — REST handlers + proxy for /api/kiosk/v1/*
│   │   ├── proxy/                 — existing forwarder (unchanged)
│   │   └── server/                — chi router + static-asset serving
│   ├── go.mod, go.sum
│   ├── Dockerfile
│   └── data/                      — runtime mount; data.db lives here
├── kiosk-spa/                     ← renamed from src/  (existing kiosk SPA)
│   └── …                          (unchanged except RESERVED_TOP_LEVEL gets "admin")
├── admin-spa/                     ← NEW
│   ├── package.json
│   ├── vite.config.ts
│   ├── tsconfig.json
│   └── src/
│       ├── main.tsx
│       ├── api/                   — fetch client w/ credentials: include
│       ├── store/auth.ts          — current user
│       ├── components/Layout.tsx, Sidebar.tsx, Modal.tsx, …
│       ├── features/
│       │   ├── login/LoginPage.tsx
│       │   ├── hotels/HotelsListPage.tsx, HotelDetailPage.tsx, HotelForm.tsx
│       │   ├── kiosks/KioskForm.tsx, KioskDetailCard.tsx (URL + QR + key)
│       │   └── audit/AuditLogPage.tsx
│       └── routes/index.tsx
├── laravel-patch/                 — unchanged
├── nginx.conf.template            — updated to route /admin and / properly
├── Dockerfile                     — multi-stage: builds both SPAs + Go binary
├── docker-compose.yml             — single service now (Go binary serves everything)
├── README.md
└── docs/
    ├── DOCUMENTATION.md           — existing review pack
    ├── PLAN_v2_ADMIN.md           — this file
    └── ADMIN_GUIDE.md             — operator-facing future-state runbook
```

Two top-level renames (`go-proxy` → `go-server`, `src/` → `kiosk-spa/`)
are cosmetic but match the "this is more than a proxy now" reality.
Done in the same commit so history stays clean.

---

## Implementation phases

### Phase 1 — DB + Go server (no UI yet, smoke via curl)

1. Add SQLite driver (`modernc.org/sqlite` for pure-Go, no CGO).
2. Schema bootstrap on first boot.
3. Repository layer (`internal/store/*.go`).
4. `cmd/admin add-user` CLI.
5. `/api/admin/v1/auth/login`, `/me`, `/logout`.
6. Hotels + kiosks CRUD endpoints.
7. Migrate the existing YAML loader to a one-shot importer (run once on
   start if `data.db` is empty AND `properties.yaml` exists; logs a
   "consider deleting properties.yaml" notice afterwards).
8. Kiosk proxy reads from DB — given UUID look up `device_key` +
   `pmsapi_url` from `kiosks` joined with `hotels`.

### Phase 2 — Admin SPA

1. New Vite project under `admin-spa/`.
2. Login page → cookie session.
3. Hotels list + detail.
4. Kiosk create form.
5. Kiosk detail card (URL + QR + key + rotate + disable).
6. Audit log page.

### Phase 3 — Kiosk SPA tweaks

Tiny diff:
- Add `'admin'` to `RESERVED_TOP_LEVEL` in `src/api/client.ts`.
- Slug regex unchanged — `k_<32 hex>` already matches.

### Phase 4 — Deployment

1. New `Dockerfile` builds both SPAs + the Go binary in stages.
2. The single Go binary serves everything (admin SPA at `/admin/*`,
   kiosk SPA at `/k_*`, APIs at `/api/admin/v1/*` and
   `/api/kiosk/v1/*`).
3. SQLite `data.db` lives in a mounted volume.
4. nginx is dropped — Go's `http.FileServer` serves the SPAs directly.
   Simpler, one fewer process, one fewer config.

### Phase 5 — Future-state docs

Two new docs alongside `DOCUMENTATION.md`:
- `ADMIN_GUIDE.md` — operator runbook (login, add hotel, add kiosk,
  rotate keys, deal with the legacy patch step).
- `OPERATIONS.md` — backup/restore SQLite, audit-log retention, how to
  add a new operator user, deploy/rollback.

---

## Test plan

### Go server (unit + integration)

- `internal/store` — table tests for each repository method against an
  in-memory SQLite.
- `internal/auth` — bcrypt verify, session expiry, sliding refresh,
  CSRF token round-trip.
- `internal/admin` — handler-level tests using `net/http/httptest` +
  cookie jar, including auth middleware.
- `internal/kiosk` — proxy still has the allowlist test; add a fixture
  test that an unknown UUID returns 404 and a disabled kiosk returns
  403.

### Admin SPA

- Vitest + Testing Library for `LoginPage`, `KioskForm`,
  `KioskDetailCard` (copy + rotate flows).
- Playwright smoke: login → add hotel → add kiosk → copy URL → URL
  responds with 200 from `/api/kiosk/v1/{uuid}/config`.

### Kiosk SPA

- Existing tests pass; add one for the `RESERVED_TOP_LEVEL` change so
  `/admin` doesn't get treated as a slug.

### End-to-end smoke (manual)

1. `docker compose up`.
2. CLI `add-user` to create the first operator.
3. Browser → `https://checkin.local/admin/login`.
4. Add a hotel pointed at a local PMSApi stub.
5. Add a kiosk, copy the URL.
6. Open the kiosk URL on a tablet, verify it loads and pulls config.
7. Rotate the key, verify the old key now returns 401.

---

## Migration from the current YAML setup

- Operators currently using `go-proxy/properties.yaml` keep it for one
  release. On first boot, if `data.db` doesn't exist and
  `properties.yaml` does, the server imports each entry into `hotels`
  + `kiosks` (one kiosk per YAML entry, slug becomes `display_name`,
  the YAML's slug is preserved as a legacy `id` so existing kiosk
  tablets keep working).
- After import the server logs:
  `imported 2 properties from properties.yaml — registry is now in data.db, you can remove the YAML once verified`.
- The YAML loader is removed in the next release.

---

## Risks / open questions

1. **Slug collisions during YAML import.** Existing kiosks live at
   `/smart-moov`. If the import keeps that slug as the kiosk id, the
   slug regex must allow legacy short slugs. Plan: import them as
   `legacy_<slug>` ids and keep an `aliases` JSON field on the kiosk
   row mapping `smart-moov → k_<new uuid>`. The Go router resolves
   either form.
2. **CSRF in admin DELETE/rotate.** SameSite=Lax + same-origin SPA is
   adequate for everything except `DELETE` and `rotate-key`. Adding a
   double-submit cookie token is one helper file; included in Phase 1.
3. **SQLite locking under load.** Admin volume is tiny (a handful of
   writes/day). Kiosk lookups are read-heavy and cached in memory.
   Should be fine; if not, swap to Postgres behind the same `Store`
   interface.
4. **Backups.** SQLite means a single `data.db` file. Operations doc
   should describe the `sqlite3 data.db ".backup '/backups/data-$(date +%F).db'"`
   cron approach.
5. **Disaster recovery for operator credentials.** If we lose the DB,
   we lose admin users. Document the `add-user` CLI as the recovery
   path; include it in OPERATIONS.md.
