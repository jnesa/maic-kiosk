# newMasterCheckin

Standalone, admin-managed, multi-tenant self check-in kiosk for hotels
running on the legacy MAIC PMS. One Docker image hosts the operator
admin panel, the guest kiosk SPA, the kiosk proxy, and a SQLite store —
no nginx, no external DB, no message broker.

```
checkin.<your>.com/admin/...        ← internal MAIC operator panel
checkin.<your>.com/k_<uuid>/...     ← guest-facing kiosk (one URL per property/group)
```

## What's in the repo

```
.
├── go-server/                     ← Go binary: admin REST + kiosk proxy + static SPA serving
│   ├── cmd/server/                — http server entrypoint
│   ├── cmd/admin/                 — operator CLI (add-user, reset-password, …)
│   └── internal/{store,auth,admin,kiosk,proxy}
├── kiosk-spa/                     ← guest kiosk SPA (React 19 + Tailwind v4)
├── admin-spa/                     ← internal admin panel (React 19 + Tailwind v4)
├── laravel-patch/                 ← drop-in for legacy PMSApi (`/api/kiosk/*` routes)
├── docs/
│   ├── PLAN_v2_ADMIN.md           — implementation plan
│   ├── SERVICE_DESIGN_v2.md       — architecture + behaviour
│   ├── ADMIN_GUIDE.md             — operator runbook
│   ├── OPERATIONS.md              — sysadmin/SRE runbook
│   └── DOCUMENTATION.md           — v1 review pack (kept for history)
├── Dockerfile                     — multi-stage build of everything
└── docker-compose.yml             — single-service deployment
```

## Quick start (Docker)

```bash
docker compose up -d --build

# create the first operator
docker compose exec checkin /app/admin add-user \
  --email you@maiccube.com --name "Your Name"

open http://localhost:8089/admin
```

The admin SPA is at `/admin`. The kiosk SPA is at `/k_<uuid>` once you
create a kiosk through the panel.

## Local dev

```bash
# Terminal 1 — Go server
cd go-server
DATA_PATH=$PWD/data/data.db ALLOWED_ORIGINS=http://localhost:5180,http://localhost:5181 \
  go run ./cmd/server

# Terminal 2 — kiosk SPA dev server
cd kiosk-spa && npm install && npm run dev    # :5180

# Terminal 3 — admin SPA dev server
cd admin-spa && npm install && npm run dev    # :5181/admin

# Terminal 4 — bootstrap an operator
cd go-server && DATA_PATH=$PWD/data/data.db go run ./cmd/admin add-user \
  --email you@local --name "Dev"
```

Vite proxies `/api/*` to the Go server on `:8089` so cookie auth works
in dev. After login you can create a hotel + kiosk through the admin
SPA, copy the URL, and open it in another tab to drive the kiosk flow.

## Onboarding a new property — high level

1. Apply the [Laravel patch](./laravel-patch/README.md) to the
   property's PMSApi (one-time per property).
2. Add the hotel via the admin panel: name + PMSApi base URL.
3. Add a kiosk under that hotel: display name + (optional)
   `legacy_group_id` + theme + languages. The panel gives you back a
   kiosk URL, a QR code, and the device key.
4. Paste `KIOSK_DEVICE_KEY` and `KIOSK_GROUP_ID` into the property's
   PMSApi `.env`, run `php artisan config:clear`.
5. Open the kiosk URL on a tablet. Done.

Full walkthrough in [`docs/ADMIN_GUIDE.md`](./docs/ADMIN_GUIDE.md).

## Architecture summary

- **No new tables on the legacy side.** Persistence reuses
  `room_reservation_guest`, `room_reservation_firm`, `prestay_history`,
  and the `room_reservation.prestay` flag — same as the existing
  `PreStayController`.
- **One SQLite file** holds operators, hotels, kiosks, sessions, and
  audit log. Mounted on `/data` so backup is `cp data.db ...`.
- **One Go binary** serves the admin REST API, the kiosk proxy, and
  both SPA bundles. nginx is not in the loop.
- **Per-kiosk device key** lives in SQLite + the property's PMSApi
  `.env`. Browser never sees it.
- **Opaque kiosk UUIDs** (`k_<32 hex>`) — unguessable, no slug fights.

Threat model + DB schema + endpoint matrix in
[`docs/SERVICE_DESIGN_v2.md`](./docs/SERVICE_DESIGN_v2.md).

## Conventions

- React 19 + TypeScript strict + Tailwind v4 on both SPAs.
- Go 1.25 with `database/sql` + `modernc.org/sqlite` (pure-Go, no CGO).
- Bcrypt for operator passwords (cost 12).
- ESLint zero-warning, Prettier defaults.

## Skills used

- `frontend-design` — bundled themes (smart-moov, pareus); plain
  professional admin chrome (slate + indigo).
- `mobile-design` — kiosk tap targets ≥ 56 px, thumb-zone CTAs.
- `simplify` — handlers stay thin; logic lives in `internal/store` and
  `internal/auth` so they can be smoke-tested without HTTP.
