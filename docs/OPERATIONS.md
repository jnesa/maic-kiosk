# Operations — newMasterCheckin

Sysadmin / SRE runbook. Covers deploy, backup, restore, monitoring, and
disaster recovery.

For the operator panel walkthrough, see
[`ADMIN_GUIDE.md`](./ADMIN_GUIDE.md). For architecture and decisions,
see [`SERVICE_DESIGN_v2.md`](./SERVICE_DESIGN_v2.md).

---

## Topology

| Component | Location |
| --- | --- |
| Single Docker image | built from the repo's root `Dockerfile` |
| HTTP server | Go binary at `/app/server`, port `8089` |
| Admin CLI | Go binary at `/app/admin` in the same image |
| SQLite | `/data/data.db` (mounted volume) |
| SPA bundles | baked in at `/app/kiosk-spa-dist` and `/app/admin-spa-dist` |

No external dependencies — no nginx, no DB server, no Redis. The
property side is unchanged: each property's PMSApi runs the
`/api/kiosk/*` patch we ship under `laravel-patch/`.

---

## Deploy (docker compose)

```bash
git pull
docker compose pull        # if you publish images to a registry
docker compose up -d --build
```

First boot creates `/data/data.db` and applies the schema. No manual
migration step.

To create the first operator (only needed once on a fresh deployment):

```bash
docker compose exec checkin /app/admin add-user \
  --email you@maiccube.com --name "Your Name"
```

That account can sign into the admin SPA and create more operators
through the CLI.

### Required environment

| Var | Required | Default | Purpose |
| --- | --- | --- | --- |
| `PORT` | no | `8089` | listen port |
| `DATA_PATH` | no | `/data/data.db` | SQLite file (must be on a persistent volume) |
| `ALLOWED_ORIGINS` | yes in prod | `https://localhost` | comma-separated CORS allowlist |
| `UPSTREAM_TIMEOUT_MS` | no | `12000` | per-request timeout to legacy PMSApi |
| `KIOSK_SPA_DIR` | no | `/app/kiosk-spa-dist` | SPA bundle path |
| `ADMIN_SPA_DIR` | no | `/app/admin-spa-dist` | SPA bundle path |

The image leaves `ALLOWED_ORIGINS` unset by default and falls back to
`https://localhost`. **Always override this in production** to the real
kiosk hostname (e.g. `https://checkin.maiccube.com`) so the cookie
session works without leaking it to other origins.

---

## Backup

SQLite means a single file. The `.backup` command is online-safe so
you don't need to stop the service.

```bash
docker compose exec checkin sh -c '
  sqlite3 /data/data.db ".backup /tmp/backup-$(date -u +%FT%H%M%SZ).db"
'
docker compose cp checkin:/tmp/backup-*.db ./backups/
```

A nightly cron on the host runs the same shape:

```cron
15 3 * * * cd /opt/newmastercheckin && \
  ts=$(date -u +\%FT\%H\%M\%SZ) && \
  docker compose exec -T checkin sh -c \
    "sqlite3 /data/data.db '.backup /tmp/$ts.db'" && \
  docker compose cp checkin:/tmp/$ts.db ./backups/$ts.db && \
  docker compose exec -T checkin rm -f /tmp/$ts.db
```

Ship the backups dir to S3 with a 30-day lifecycle policy. SQLite
backups are tiny (few MB), so this is essentially free.

> ⚠️ The image doesn't ship `sqlite3`. Either install it inside the
> image (one Alpine apk line in the Dockerfile) or copy the file off
> the volume on the host instead — `docker run --rm -v
> kiosk-data:/data alpine sh -c 'cp /data/data.db /backup/...'`.

---

## Restore

1. Stop the service: `docker compose stop checkin`.
2. Replace the file:
   ```bash
   docker compose cp ./backups/2026-04-27T0315Z.db checkin:/data/data.db
   ```
3. `docker compose start checkin`.
4. Sessions older than 12 hours are auto-purged on first request; users
   sign in again as needed.

If `/data/data.db` is unrecoverable and you have no backup: ops can
recreate the schema on first boot and re-run `admin add-user`. Hotels
and kiosks must then be re-registered manually. **This is why backups
matter.**

---

## Disaster recovery

| Failure | Recovery |
| --- | --- |
| Container won't start | Logs first: `docker compose logs --tail=200 checkin`. Common: corrupt `data.db` (restore from backup) or `DATA_PATH` permission mismatch. |
| All operators locked out | Run the CLI directly against the volume: `docker run --rm -it -v kiosk-data:/data newmastercheckin:dev /app/admin add-user --email …`. |
| Lost operator password | Another operator runs `reset-password`. If no operator can log in, see above. |
| Property's PMSApi down | Kiosks for that property show a guest-facing error after timeout. No action on our side — the property side is independent. |
| Kiosk service crashed under load | Restart the container. SQLite WAL recovers in milliseconds. There's no in-memory state to lose. |

---

## Monitoring

### Liveness / readiness

| Endpoint | Use |
| --- | --- |
| `GET /api/kiosk/v1/health` | k8s liveness — 200 if process is alive |
| `GET /api/kiosk/v1/ready` | k8s readiness — pings the DB |

Hook these into your platform's probes. The Compose file defines a
`HEALTHCHECK` that polls `/api/kiosk/v1/health` every 30 s.

### Logs

JSON-structured request logs from chi's middleware. Recommended host-level
filters:

- `path=/api/admin/v1/auth/login` — login attempts. Watch for spikes.
- `status=5xx` AND `path~^/api/kiosk/v1/.+/(lookup|submit)` — proxy
  failures from the legacy side. Don't alert below 1%/min; alert above
  5%/min for any single kiosk.
- `path=/api/kiosk/v1/<id>/config status=410` — disabled-kiosk traffic.
  Useful for detecting stale tablets the property forgot to retire.

### Metrics (future)

Out of scope for v2. When we add them, the natural shape is a
Prometheus `/metrics` endpoint counting:

- `kiosk_proxy_requests_total{kiosk_id,path,status}`
- `admin_login_attempts_total{result}`
- `admin_audit_writes_total{action}`

---

## Rotating secrets

### A property's device key

This is operator-driven via the admin panel. See the *Rotate a device
key* section of [`ADMIN_GUIDE.md`](./ADMIN_GUIDE.md). The legacy `.env`
must be updated by the same operator. Rotation is fast (one click +
one `php artisan config:clear` on the legacy side).

### An operator's password

```bash
docker compose exec checkin /app/admin reset-password --email user@…
```

All sessions for that user are revoked.

### The session cookie secret

Sessions are random opaque tokens — there's no signing key to rotate.
Cookies inherit their security from `HttpOnly`, `Secure`, and
`SameSite=Lax` settings.

---

## Adding capacity

SQLite handles ~1k writes/sec on a laptop SSD; admin volumes are
tiny (a few writes per minute). Kiosk reads are read-heavy — every
guest's check-in flow does ~10 lookups by ID — but they hit a ~1 KB
row by primary key.

Threshold to revisit: ~500 active properties or ~100 concurrent
check-ins. At that scale we'd:

1. Keep the same Go binary.
2. Swap `internal/store/store.go` to open Postgres instead of SQLite
   (the `Store` interface is unchanged from a handler's perspective).
3. Move the schema to a migration tool (`golang-migrate` or `goose`).

No code refactor required outside `internal/store/`.

---

## Audit log retention

`audit_log` is unbounded today. We keep the latest 12 months by
default. To prune, run:

```sql
DELETE FROM audit_log WHERE created_at < strftime('%Y-%m-%dT%H:%M:%fZ', 'now', '-12 months');
VACUUM;
```

Wrap that in a nightly cron when volume warrants it. Pre-prune,
optionally export to JSON for archival:

```sql
.mode json
.output /tmp/audit-archive-2025.json
SELECT * FROM audit_log WHERE created_at < strftime('%Y-%m-%dT%H:%M:%fZ', 'now', '-12 months');
.output stdout
.mode column
```

---

## Local dev (no Docker)

The Go binary and either SPA can run independently for fast iteration.

```bash
# Terminal 1 — Go server
cd go-server
DATA_PATH=$PWD/data/data.db ALLOWED_ORIGINS="http://localhost:5180,http://localhost:5181" \
  go run ./cmd/server

# Terminal 2 — kiosk SPA
cd kiosk-spa
npm install && npm run dev      # http://localhost:5180

# Terminal 3 — admin SPA
cd admin-spa
npm install && npm run dev      # http://localhost:5181/admin

# Terminal 4 — first operator
cd go-server
DATA_PATH=$PWD/data/data.db go run ./cmd/admin add-user \
  --email you@local --name "Dev"
```

Both Vite dev servers proxy `/api/*` to `http://localhost:8089` so the
cookie session works.

---

## Out of scope (intentionally)

- **No CDN.** Static SPA assets are served by the Go binary. Add a CDN
  in front of `checkin.<your>.com` only if traffic warrants it.
- **No multi-region.** SQLite is one box. If you need geographic
  redundancy, swap to Postgres + Litestream (or just Postgres).
- **No SSO for operators.** Per-user accounts in our DB. Adding OIDC
  is a small change but not in scope here.
- **No automated property onboarding.** The legacy patch has to be
  installed manually per property; we can't automate it without admin
  access to each property's host.
