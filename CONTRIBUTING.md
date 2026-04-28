# Contributing

Engineering quick-reference for the MAIC Kiosk service. For deeper context,
see [`docs/SERVICE_DESIGN_v2.md`](./docs/SERVICE_DESIGN_v2.md).

## Dev loop

You don't need Docker for everyday work. Run the three pieces directly:

```bash
# Terminal 1 — Go server (admin REST + kiosk proxy + static SPA serve)
cd go-server
DATA_PATH=$PWD/data/data.db \
  ALLOWED_ORIGINS=http://localhost:5180,http://localhost:5181 \
  go run ./cmd/server                                       # :8089

# Terminal 2 — kiosk SPA dev server
cd kiosk-spa && npm install && npm run dev                  # :5180

# Terminal 3 — admin SPA dev server
cd admin-spa && npm install && npm run dev                  # :5181/admin

# Terminal 4 — bootstrap an operator
cd go-server && DATA_PATH=$PWD/data/data.db \
  go run ./cmd/admin add-user --email you@local --name "Dev"
```

Vite proxies `/api/*` → `http://localhost:8089` so cookie auth works in
dev. Both SPAs hot-reload; the Go server picks up changes on `go run`
restart.

## Layout

```
go-server/      Go service. Touch this for backend changes.
  cmd/server    HTTP entrypoint
  cmd/admin     Operator CLI
  internal/
    store/      SQLite repository (one file per entity)
    auth/       bcrypt + sessions + middleware
    admin/      /api/admin/v1/* handlers
    kiosk/      /api/kiosk/v1/* handlers + slug resolver
    proxy/      Allowlisted forwarder to legacy PMSApi

kiosk-spa/      Guest-facing SPA. React 19, Tailwind v4, Zustand.
admin-spa/      Internal operator panel. React 19, Tailwind v4, React Query.
laravel-patch/  PHP files that drop into a property's PMSApi.
docs/           Design + operator + ops docs.
```

## Conventions

- **TS strict** — both SPAs. `noUnusedLocals`, `noUnusedParameters`,
  `noFallthroughCasesInSwitch` are on.
- **ESLint zero-warning** — both SPAs. CI fails on warnings.
- **Go modules** — pure-Go SQLite (`modernc.org/sqlite`), no CGO. Don't
  introduce a CGO dependency without a strong reason — it doubles the
  build time and breaks the Alpine image.
- **`internal/*` packages** — each one owns its concern. Handlers stay
  thin; logic lives in `store/` and `auth/` so it can be tested without
  HTTP.
- **No new tables on the legacy side.** This is the whole point of the
  Laravel patch. Touch `room_reservation_guest`, `room_reservation_firm`,
  `prestay_history`, and `room_reservation.prestay`. That's it.

## Adding a property theme

Themes are bundled into the kiosk SPA. Adding a new brand is two files:

1. `kiosk-spa/src/theme/themes/<id>.ts` — copy `smart-moov.ts` and tune
   the palette + fonts.
2. Register the theme id in `kiosk-spa/src/theme/index.ts`.

Then drop hero/logo assets at `kiosk-spa/public/themes/<id>/` and pick
the new theme id when creating a kiosk in the admin panel.

## Adding a new API endpoint

1. Define the wire shape (request + response) inline in the handler.
   Don't reach for a heavy DTO library — these handlers are 30 lines.
2. Add the handler in `internal/admin/http.go` or
   `internal/kiosk/http.go`.
3. Mount the route in the same file's `Mount(r chi.Router)` method.
4. If the endpoint mutates state, call `h.audit(...)` before responding
   so the action lands in `audit_log`. Strip secrets from the payload —
   never log device keys or password hashes.
5. Add the corresponding fetch helper in `admin-spa/src/api/` or
   `kiosk-spa/src/api/`. Keep them tiny (5–10 lines each).

## Tests

There are no automated tests today. Recommended scaffolding when we add
them:

- **Go unit** — `internal/store` has a clear interface; spin up an
  in-memory SQLite (`:memory:`) for table tests.
- **Go integration** — `net/http/httptest` + a cookie jar for the admin
  flows. The proxy has a clean `Target` shape so a fake upstream is
  trivial.
- **Playwright** — login → add hotel → add kiosk → copy URL → open
  kiosk SPA → see Welcome screen. Single happy-path smoke is enough to
  catch most regressions.

## Building production artefacts

The single Dockerfile builds both SPAs and the Go binary in stages.
Locally:

```bash
docker compose build
docker compose up -d
```

CI should run on every PR:

```bash
# go-server
cd go-server && go build ./... && go vet ./... && go test ./...

# kiosk-spa
cd kiosk-spa && npm ci && npm run lint && tsc --noEmit && npm run build

# admin-spa
cd admin-spa && npm ci && npm run lint && tsc --noEmit && npm run build
```

## Release flow

1. PR against `main` of `MaicSystem/maic-kiosk`. Reviewer checks the
   Go diff, both SPA diffs, and any docs touched.
2. After merge, tag the release: `git tag -a v0.x -m "..." && git push --tags`.
3. The orchestrator's `docker-compose.yml` builds from
   `../Kiosk` (sibling clone), so a release on the kiosk repo is picked
   up by the next orchestrator deploy.

## Skills used

- `frontend-design` — bundled themes (smart-moov, pareus); plain
  professional admin chrome.
- `mobile-design` — kiosk tap targets ≥ 56 px.
- `simplify` — handlers stay thin, logic in `internal/{store,auth}`.
