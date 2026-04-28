# Service Design — newMasterCheckin (v2, admin-managed)

How the kiosk works after the v2 admin-panel changes land. This doc is
the source of truth for behaviour, contracts, and operational
expectations once Phase 1–4 from `PLAN_v2_ADMIN.md` ship.

If you're looking for the v1 (YAML-only) review pack, see
[`DOCUMENTATION.md`](./DOCUMENTATION.md).

---

## Overview in one paragraph

A standalone, multi-tenant self check-in kiosk service hosted on its
own domain (`checkin.maiccube.com`). MAIC operators register hotels —
each backed by one of the legacy PMSApi subdomains — and create kiosk
links per hotel. A hotel with multiple `g_group` rows gets one link
per group; a single-tenant subdomain gets one link, period. Each link
is an opaque UUID. The browser only ever talks to our Go server; the
per-link device key + PMSApi URL stay in our SQLite database, never
on the wire.

---

## System map

```
Operator's browser            Guest's tablet at the property
  │                            │
  │ /admin/...                 │ /k_<uuid>/...
  ▼                            ▼
                       checkin.maiccube.com
                       ┌────────────────────────────────────────┐
                       │  Go server (single binary)             │
                       │   • serves admin SPA at /admin/*       │
                       │   • serves kiosk SPA at /k_*           │
                       │   • /api/admin/v1/* (sessioned)        │
                       │   • /api/kiosk/v1/* (UUID-routed)      │
                       │                                        │
                       │   data.db (SQLite, mounted volume)     │
                       │     hotels, kiosks, admin_users,       │
                       │     admin_sessions, audit_log          │
                       └─────────────────┬──────────────────────┘
                                         │ HTTPS + X-Device-Key
                                         ▼
                              Per-property legacy PMSApi
                              with /api/kiosk/* patch installed
```

---

## Roles

| Role | Auth | Access |
| --- | --- | --- |
| Operator (MAIC internal) | email + password (bcrypt), session cookie | full admin panel |
| Guest at kiosk | none — UUID is the entire authorisation | their property's check-in flow only |
| Property staff | none — they receive the kiosk URL out-of-band and configure their PMSApi `.env` | not a user of this service |

The kiosk URL is unguessable (32 hex chars after the `k_` prefix), so
"unauthenticated guest with the URL" is the access pattern by design.
Disabling a kiosk in the admin panel is the kill-switch.

---

## Data model

```
admin_users      one row per MAIC operator
admin_sessions   active login sessions (cookie token → user)
hotels           one row per legacy PMSApi subdomain
kiosks           one row per check-in URL
audit_log        every admin write action
```

A **hotel** is a connection to a legacy PMSApi backend. A **kiosk** is
a tablet-facing URL bound to a hotel, optionally scoped to one
`g_group.id` on that hotel's PMSApi.

| Cardinality | Example |
| --- | --- |
| 1 hotel : 1 kiosk | smart moov hotel — single tenant, one lobby tablet |
| 1 hotel : N kiosks | "Schladming" subdomain hosts 4 properties; each gets its own kiosk URL |

Full SQL is in
[`PLAN_v2_ADMIN.md#data-model`](./PLAN_v2_ADMIN.md#data-model-sqlite).

---

## URL contract

| Pattern | Purpose | Example |
| --- | --- | --- |
| `/` | Redirects to `/admin/login` | — |
| `/admin/...` | Admin SPA | `/admin/hotels/12` |
| `/api/admin/v1/...` | Admin REST API | `POST /api/admin/v1/hotels` |
| `/api/kiosk/v1/{uuid}/...` | Public kiosk REST API | `POST /api/kiosk/v1/k_8a3f.../lookup` |
| `/k_<uuid>` and `/k_<uuid>/...` | Kiosk SPA (one per kiosk) | `/k_8a3f.../checkin/1` |
| `/health` | Liveness | — |

Reserved top-level paths the kiosk SPA never treats as a UUID:
`admin`, `api`, `assets`, `locales`, `health`.

---

## Operator workflows

### Add a single-tenant hotel + kiosk

1. **Login** at `/admin/login` (operator's email + password).
2. **Hotels → New hotel.** Enter name and the legacy `pmsapi_url`
   (e.g. `https://pms.maiccube.com`). Save.
3. **Open the hotel → New kiosk.**
   - Display name: human-readable, e.g. `smart moov — lobby`.
   - Legacy group id: leave blank if the property doesn't use groups.
   - Theme + languages.
   - Optional hero image / logo / support contact.
4. The detail card shows:
   - The kiosk URL (`/k_<uuid>`) — copy / QR.
   - The device key — copy.
   - Reminders to set `KIOSK_DEVICE_KEY` and `KIOSK_GROUP_ID` on the
     legacy PMSApi `.env`.
5. The operator sets those two env vars on the property's PMSApi,
   restarts php-fpm, and hands the URL to the property.

### Add a multi-group hotel

Same as above, but step 3 is repeated once per group. Each kiosk
inherits the same `pmsapi_url` (one hotel row, many kiosks under it)
but each has its own `legacy_group_id`, its own theme/languages if
desired, and its own device key.

### Rotate a device key

`Kiosks → … → Rotate key` regenerates `kiosks.device_key` and shows
the new value once. The old key stops working immediately on our side;
the operator must update the property's PMSApi `.env` and restart
php-fpm before the kiosk works again.

### Disable a kiosk

`Kiosks → … → Disable` flips `status='disabled'`. Subsequent requests
to `/api/kiosk/v1/{uuid}/...` return `410 Gone`. The kiosk SPA shows
"This kiosk has been deactivated. Please see reception." Re-enable
restores normal operation; nothing has to change on the legacy side.

### Delete a hotel

Cascade-deletes all its kiosks. Audit-logged. The legacy PMSApi side
isn't notified — operators should disable the corresponding `KIOSK_*`
env vars there for cleanliness.

---

## API contracts (admin)

All paths are under `/api/admin/v1`. All except `/auth/login` require
a valid session cookie.

```http
POST /auth/login
Body: {"email":"ilija@maiccube.com","password":"…"}
Response 200: {"user":{"id":1,"email":"…","name":"…"}}
Sets cookie: kiosk_admin_session=<32-byte-hex>; HttpOnly; Secure; SameSite=Lax; Path=/

POST /auth/logout
Response 204; clears cookie + revokes session row

GET  /me
Response 200: {"id":1,"email":"…","name":"…"}

GET  /hotels
Response 200: {"hotels":[{"id":1,"name":"smart moov","pmsapi_url":"…","kiosk_count":1},…]}

POST /hotels
Body: {"name":"Schladming Group","pmsapi_url":"https://schladming.maiccube.com","notes":"…"}
Response 201: {"id":2,…}

GET  /hotels/{id}
Response 200: {"id":1,…,"kiosks":[{…},…]}

PATCH /hotels/{id}
Body: any subset of {name, pmsapi_url, notes}
Response 200: updated hotel

DELETE /hotels/{id}
Response 204 (cascade delete kiosks)

POST /hotels/{id}/kiosks
Body: {
  "display_name":"smart moov — lobby",
  "legacy_group_id":72,
  "legacy_group_label":"smart moov",
  "theme":"smart-moov",
  "languages":["en","de","it"],
  "hero_image":"/themes/smart-moov/hero.jpg",
  "logo":"/themes/smart-moov/logo.svg",
  "support_phone":"+39 …",
  "support_email":"reception@…"
}
Response 201: {
  "id":"k_8a3f9c1b…",
  "url":"https://checkin.maiccube.com/k_8a3f9c1b…",
  "device_key":"9d3feacf…",     // shown ONCE in the response
  …
}

GET   /kiosks/{id}            full kiosk row, incl. device_key for re-display
PATCH /kiosks/{id}            update editable fields (not device_key)
POST  /kiosks/{id}/rotate-key  regenerates device_key, returns the new value
POST  /kiosks/{id}/disable    sets status='disabled'
POST  /kiosks/{id}/enable     sets status='active'
DELETE /kiosks/{id}           hard delete

GET /audit-log?limit=50&before=<id>
Response 200: {"entries":[{"id":…,"actor":"ilija@maiccube.com","action":"create_kiosk","entity_id":"k_…","payload":{…},"created_at":"…"},…]}
```

Every admin write returns the audit-log entry id in a response header
(`X-Audit-Id`) so the SPA can deep-link to the action.

---

## API contracts (public kiosk)

Unchanged from v1 in shape, only routing changes (UUID instead of
slug).

```http
GET /api/kiosk/v1/{uuid}/config
Response 200: {
  "id":"k_8a3f…",
  "name":"smart moov",
  "subtitle":"hotel",
  "theme":"smart-moov",
  "languages":["en","de","it"],
  "heroImage":"/themes/smart-moov/hero.jpg",
  "logo":"/themes/smart-moov/logo.svg",
  "supportPhone":"+39 …",
  "supportEmail":"reception@…"
}
Response 410 if status='disabled'
Response 404 if uuid unknown

POST /api/kiosk/v1/{uuid}/lookup       → forwarded to {hotel.pmsapi_url}/api/kiosk/lookup
POST /api/kiosk/v1/{uuid}/select       → forwarded to /api/kiosk/select
POST /api/kiosk/v1/{uuid}/form         → forwarded to /api/kiosk/form
POST /api/kiosk/v1/{uuid}/save-guest   → forwarded to /api/kiosk/save-guest
POST /api/kiosk/v1/{uuid}/save-firm    → forwarded to /api/kiosk/save-firm
POST /api/kiosk/v1/{uuid}/submit       → forwarded to /api/kiosk/submit
```

The proxy adds `X-Device-Key: <kiosks.device_key>` and
`X-Forwarded-For: <client ip>` on the way out. The legacy
`KioskController` then enforces `KIOSK_GROUP_ID` to keep one kiosk
from reaching another property's data.

---

## Security model

| Concern | Mitigation |
| --- | --- |
| Operator account compromise | bcrypt-hashed passwords, sessions revocable, audit-log of every write. |
| Session theft (XSS) | `HttpOnly`, `Secure`, `SameSite=Lax` cookie. CSP on the admin SPA. |
| CSRF on state-changing admin calls | `SameSite=Lax` + same-origin SPA covers most. Double-submit cookie token added on `DELETE` and `rotate-key`. |
| Brute force on `/auth/login` | Per-IP rate limit (10/min), per-email lock (5 failures → 15-min lock). Audit-logged. |
| Kiosk URL guessing | UUID is 128 random bits; the search space is large enough that bruteforce is uneconomical. The hotel must still validate the device key, so even a guessed UUID would fail at the legacy side. |
| Disabled kiosk still hit by bookmark | `/config` returns 410; SPA shows the deactivated screen. |
| Cross-tenant escape | Each kiosk has its own device key. Legacy `KIOSK_GROUP_ID` per property pins what that key can see. |
| SQLite file leak | `data.db` permissions 0600 (mounted-volume convention). Backups encrypted at rest. |

What we explicitly accept:

- Anyone with the kiosk URL (no auth) can drive a check-in. That's the
  whole product. Kill-switch is the disable button.
- Operators with admin access can read every hotel's data. We're
  internal. If/when we add external customer admins, this changes.

---

## Audit log

Every state-changing admin call writes one row to `audit_log` with:

- `admin_user_id` — who did it
- `action` — `create_hotel`, `update_hotel`, `delete_hotel`,
  `create_kiosk`, `update_kiosk`, `rotate_key`, `disable_kiosk`,
  `enable_kiosk`, `delete_kiosk`, `login`, `logout`
- `entity_type` — `hotel`, `kiosk`, `admin_user`
- `entity_id` — the row id (or kiosk UUID)
- `payload` — JSON, secrets stripped (device keys are never logged)
- `created_at`

Retention: 12 months by default. A nightly job rotates rows older than
that into `data/audit-archive/<year>-<month>.json` then deletes them.

---

## Theme and asset story

Themes are bundled into the kiosk SPA build (one bundle, all themes).
The active theme is decided at runtime by `kiosks.theme` returned from
`/config`. Bundled themes today: `smart-moov`, `pareus`. Adding a new
brand is a code change (new theme manifest in
`kiosk-spa/src/theme/themes/<id>.ts` + register in `theme/index.ts`),
not a config change. That's deliberate — themes are real design work,
not data.

Hero/logo assets are stored in `public/themes/<id>/` and referenced by
URL in `kiosks.hero_image` and `kiosks.logo`. A property can override
the bundled assets by uploading their own to a CDN and pasting the URL
in the admin form, but the *theme* (palette, fonts) stays bundled.

---

## Operations

### Backup

Nightly `cron`:

```bash
sqlite3 /data/data.db ".backup '/backups/data-$(date +%F).db'"
```

Off-box: ship to S3 with a 30-day lifecycle policy.

### Restore

Stop the service, copy the backup file over `/data/data.db`, restart.
Sessions are preserved if recent enough (12-h sliding expiry); admin
users can re-login otherwise.

### Add an operator

```bash
docker exec checkin-server admin-cli add-user \
  --email luca@maiccube.com --name "Luca Rossi"
# Prompts for the password, bcrypt-hashes, inserts a row.
```

### Reset an operator's password

```bash
docker exec -it checkin-server admin-cli reset-password \
  --email luca@maiccube.com
```

Audit-logged with `action=reset_password`, actor = the CLI user.

### Disable an operator

```bash
docker exec checkin-server admin-cli disable-user --email luca@…
```

Sets a row-level flag and revokes any active sessions for that user.

### Health probes

| Endpoint | Used by |
| --- | --- |
| `GET /health` | k8s liveness — returns 200 if process is alive |
| `GET /api/kiosk/v1/ready` | k8s readiness — pings the DB and reports kiosk count |

### Logs

JSON-structured, one line per request. Sensitive fields (`password`,
`device_key`, `Authorization`, `Cookie`) are scrubbed before logging.
Kiosk submit-flow logs include `kiosk_id` and `hotel_id` so an
operator can find the relevant audit-log row from a guest complaint.

### Deploy

`docker compose pull && docker compose up -d`. The Go server runs
SQLite migrations at boot if the schema version is older than the
binary. SPA bundles are baked into the image at build time, so SPA
changes are a redeploy.

---

## What stays simple

- One Go binary, one SQLite file, two SPA bundles. No Postgres, no
  Redis, no message broker. Kiosk volume is small (<10 properties at
  launch, <100 forecast for the next 18 months). When SQLite stops
  being enough, swap to Postgres behind the same `Store` interface;
  no handler changes needed.
- nginx is gone. The Go binary serves the SPA bundles directly with
  `http.FileServer` plus a custom 404 handler that falls through to
  `index.html` for SPA routes. One fewer process, one fewer config
  file.
- No webhooks, no async jobs, no queue. Every admin action is
  synchronous; every kiosk submit is a chain of synchronous proxy
  calls to the legacy PMSApi.

---

## What we owe the operator (out of scope here)

The legacy side still needs the `/api/kiosk/*` patch installed once
per property and `KIOSK_DEVICE_KEY` + `KIOSK_GROUP_ID` set in `.env`.
This is fundamental — without those, the proxy can't talk to PMSApi
at all. The admin panel reminds the operator with a checklist on the
kiosk detail card; we don't (and can't) automate the legacy side.

A future iteration could add an SSH-based "auto-install patch on a
property" tool. Out of scope for v2.

---

## Glossary

- **Hotel** — one row in `hotels`. Maps 1:1 to a legacy PMSApi
  subdomain.
- **Kiosk** — one row in `kiosks`. One URL, one device key, optionally
  scoped to one `g_group.id` on the legacy PMSApi.
- **Group** — `g_group.id` on the legacy side. Multiple groups within
  one PMSApi instance share a database but represent separate
  properties in the MAIC tenancy model.
- **Property** — colloquially used for both hotels and groups. In this
  doc, "property" maps to "hotel + group" (a kiosk's effective scope).
- **UUID / kiosk id** — `k_<32 hex chars>`, the public URL identifier.
- **Device key** — random hex secret stored in `kiosks.device_key`,
  echoed in `KIOSK_DEVICE_KEY` on the property's PMSApi `.env`.
