# newMasterCheckin — Documentation & Review Pack

A reviewer-oriented walkthrough of the multi-tenant self check-in kiosk.
Reads top-to-bottom for context; the **Review Checklist** at the end is
the actionable summary if you'd rather start there.

---

## Table of Contents

1. [What this is](#what-this-is)
2. [System architecture](#system-architecture)
3. [Design decisions and trade-offs](#design-decisions-and-trade-offs)
4. [Repository layout](#repository-layout)
5. [Property registry & onboarding](#property-registry--onboarding)
6. [API surface](#api-surface)
7. [Data flow per check-in](#data-flow-per-check-in)
8. [Theme system](#theme-system)
9. [Internationalisation](#internationalisation)
10. [Security model](#security-model)
11. [Legacy PMSApi patch](#legacy-pmsapi-patch)
12. [Deployment](#deployment)
13. [Verification](#verification)
14. [Open issues and follow-ups](#open-issues-and-follow-ups)
15. [Review checklist](#review-checklist)

---

## What this is

A standalone, **multi-property** self check-in kiosk for hotels running on
the legacy MAIC PMS. One deployment can serve any number of properties,
each routed by URL path.

| Concern | Choice |
| --- | --- |
| Hosting | One domain (e.g. `checkin.maiccube.com`) |
| Property routing | URL path prefix — `/<slug>/...` |
| Frontend | React 19 SPA, served by nginx |
| Backend | Stateless Go proxy that holds per-property device keys |
| Persistence | None of its own — writes through to legacy PMSApi |
| New DB tables | **None.** Reuses `room_reservation_guest`, `room_reservation_firm`, `prestay_history`, `room_reservation.prestay` |

The browser only ever talks to the proxy. The proxy adds the
per-property `X-Device-Key` and forwards to that property's PMSApi. The
device key never reaches the browser.

---

## System architecture

```
                                        checkin.maiccube.com (this repo)
                                        ┌──────────────────────────────────────┐
                                        │                                      │
   Browser at                           │  ┌────────────────┐                  │
   checkin.maiccube.com/smart-moov  ─▶  │  │   nginx        │                  │
                                        │  │   serves SPA   │                  │
                                        │  │   /api/* →     │                  │
                                        │  │    proxy:8089  │                  │
                                        │  └───────┬────────┘                  │
                                        │          │                           │
                                        │          ▼                           │
                                        │  ┌──────────────────┐                │
                                        │  │ checkin-kiosk-   │  reads at      │
                                        │  │ api (Go)         │◀── boot ── properties.yaml
                                        │  │                  │                │
                                        │  │ 1. resolve slug  │                │
                                        │  │ 2. pick property │                │
                                        │  │ 3. add X-Device- │                │
                                        │  │    Key           │                │
                                        │  │ 4. forward       │                │
                                        │  └────────┬─────────┘                │
                                        │           │                          │
                                        └───────────┼──────────────────────────┘
                                                    │
                                                    ▼  HTTPS + X-Device-Key
                                              ┌───────────────────────────┐
                                              │  Legacy PMSApi            │
                                              │  pms.maiccube.com         │
                                              │  /api/kiosk/lookup …      │
                                              │  /api/kiosk/submit        │
                                              └───────────┬───────────────┘
                                                          │
                                                          ▼
                                              ┌───────────────────────────┐
                                              │  testSchladming MariaDB   │
                                              │  room_reservation,        │
                                              │  room_reservation_guest,  │
                                              │  room_reservation_firm,   │
                                              │  prestay_history          │
                                              └───────────────────────────┘
```

**Trust boundaries:**

- Browser ↔ proxy: anyone can hit the proxy. Per-property routing is
  the only authorisation needed for `/config`. Mutating endpoints
  (lookup, submit, etc.) gain their authorisation from the legacy
  PMSApi side once the device key is added.
- Proxy ↔ legacy PMSApi: device-key authenticated. The legacy side
  rejects requests without `X-Device-Key`.
- Per property: `KIOSK_GROUP_ID` on the legacy side ensures even with
  a valid device key the kiosk can only see/write its own property's
  reservations.

---

## Design decisions and trade-offs

### Why path-based property routing

We considered four mechanisms (subdomain, path, custom domain, device
pairing token). Path-based won because:

- One TLS certificate covers everything.
- One DNS row covers everything.
- Adding a property is purely a YAML edit + env var; no DNS or cert work.
- Reception can deep-link a tablet to `/<slug>` from a printed setup
  card.

Trade-off: the slug is visible in URLs. Acceptable — slugs aren't
secret. Reservation IDs, names, and DOBs never appear in URLs.

### Why a Go proxy in front, not browser-direct

If the SPA called the legacy `/api/kiosk/*` endpoints directly:

- The device key would have to be in the browser bundle (or fetched
  per-session, which still ends up in memory).
- Every property would need CORS allowlist updates on the legacy side.
- A misbehaving kiosk page (XSS, supply-chain) could exfiltrate the key.

With the proxy:

- Device keys live in env vars on the kiosk host, never in the browser.
- Each property's PMSApi only needs to allow the proxy's IP/origin.
- Allowlist of six legacy endpoints (`proxy/proxy.go:Allowed`) — even
  if the proxy is exploited, it can't be used as a generic open relay.

### Why no new database tables

The legacy `PreStayController` already writes per-guest data to
`room_reservation_guest`, firm/extras + signature to
`room_reservation_firm`, and tracks the visit in `prestay_history`. The
kiosk uses the same columns with `prestay_history.type='kiosk'` so all
existing dashboards, automation, and Feratel pickup pipelines keep
working unchanged. Migrations are risky on a live monolith; this avoids
them entirely.

### Why one SPA build for all properties

The alternative — one container per property, each built with a
property-specific theme/secret — works but doesn't scale. Every
property change becomes an image rebuild and a redeploy. With runtime
property resolution, a new property is one PR + one env secret + a
restart. Bundled themes still ship in every build (small cost) but the
active one is selected when the property's `/config` resolves.

### Why a YAML registry, not a DB

For 5–50 properties, a checked-in file is auditable, atomic, and
doesn't require a schema or migration plan. If the property count grows
past that, swap the loader for a DB-backed implementation behind the
same `Lookup(slug)` interface in `internal/config/config.go` — no
handler or proxy changes needed.

---

## Repository layout

```
.
├── README.md                    — quick start
├── docs/DOCUMENTATION.md        — this file
├── package.json                 — React 19, Vite, Tailwind v4, RHF, Zod, Zustand
├── tsconfig.json
├── vite.config.ts
├── Dockerfile                   — node build → nginx (SPA only)
├── nginx.conf.template          — SPA fallback + /api proxy
├── docker-compose.yml           — pair: SPA + go-proxy
├── public/
│   ├── locales/{en,de,it,fr}/   — i18n bundles
│   └── themes/{smart-moov,pareus}/  — drop hero.jpg + logo.svg
├── src/
│   ├── api/                     — axios client (slug-aware), wire types
│   │   ├── client.ts            — propertySlug() reads window.location.pathname
│   │   ├── property.ts          — fetchProperty() → /config
│   │   ├── lookup.ts, checkin.ts, types.ts
│   ├── store/                   — Zustand
│   │   ├── property.ts          — active property + boot status
│   │   ├── session.ts           — current reservation
│   │   ├── lookup.ts            — Stage A → B candidate token
│   │   └── form.ts              — in-progress check-in form
│   ├── theme/                   — bundled property themes
│   │   ├── types.ts
│   │   ├── index.ts             — applyTheme(), resolveTheme()
│   │   └── themes/{smart-moov,pareus}.ts
│   ├── components/
│   │   ├── PropertyProvider.tsx — gates SPA on /config fetch
│   │   ├── KioskShell.tsx       — header + layout
│   │   ├── Wordmark.tsx
│   │   ├── LanguageSwitcher.tsx
│   │   ├── SignaturePad.tsx     — HTML5 canvas, pointer events
│   │   ├── StepProgress.tsx
│   │   ├── IdleResetGuard.tsx   — 90s idle reset with 10s countdown
│   │   ├── ErrorScreen.tsx, Field.tsx
│   ├── features/
│   │   ├── landing/
│   │   │   ├── WelcomePage.tsx          — full-bleed hero, "Begin check-in"
│   │   │   ├── LookupLastNamePage.tsx   — Stage A
│   │   │   ├── LookupPickGuestPage.tsx  — Stage B (ambiguous)
│   │   │   ├── LookupByIdPage.tsx       — manual fallback
│   │   │   └── LookupNotFoundPage.tsx
│   │   ├── checkin/
│   │   │   ├── CheckinLayout.tsx
│   │   │   ├── Step1GuestPage.tsx
│   │   │   ├── Step2FirmPage.tsx
│   │   │   └── Step3ReviewPage.tsx
│   │   ├── done/DonePage.tsx
│   │   └── error/ErrorPage.tsx
│   ├── i18n/index.ts            — i18next-http-backend
│   ├── router/index.tsx         — buildRouter(basename)
│   ├── main.tsx                 — boot
│   └── index.css                — Tailwind + theme tokens
├── go-proxy/                    — Go multi-tenant proxy (sibling)
│   ├── cmd/main.go
│   ├── internal/
│   │   ├── config/config.go     — loads + validates properties.yaml
│   │   ├── proxy/proxy.go       — allowlisted forwarder
│   │   └── server/              — chi router + slug middleware
│   ├── properties.example.yaml
│   ├── Dockerfile
│   ├── docker-compose.yml
│   └── k8s/deployment-dev.yaml
└── laravel-patch/               — drop-in for legacy PMSApi
    ├── README.md                — Plesk SSH install steps
    └── payload/
        └── app/Http/
            ├── Middleware/KioskDeviceKey.php
            └── Controllers/API/KioskController.php
```

---

## Property registry & onboarding

`go-proxy/properties.yaml` is the single source of truth for which
properties this kiosk serves.

```yaml
properties:
  - slug: smart-moov                       # URL path segment
    name: smart moov
    subtitle: hotel
    pmsapi_url: https://pms.maiccube.com   # legacy backend base URL
    device_key: ${SMART_MOOV_DEVICE_KEY}   # env interpolation
    theme: smart-moov                      # one of the bundled themes
    languages: [en, de, it]
    hero_image: /themes/smart-moov/hero.jpg
    logo: /themes/smart-moov/logo.svg
    support_phone: "+39 0421 1830 350"
    support_email: reception@smart-moov-hotel.com
```

### Validation (enforced at boot — `internal/config/config.go::validate`)

- `slug`: regex `^[a-z0-9][a-z0-9-]{1,38}[a-z0-9]$` (3–40 chars,
  matches what's safe in a URL path).
- `pmsapi_url`: must parse as `http(s)://...` with a host.
- `device_key`: required; `${VAR}` references must resolve.
- `theme`: free string at the proxy level; the SPA falls back to
  `smart-moov` if it can't find a matching manifest, so an unknown
  theme name doesn't break the kiosk.
- Duplicate `slug` → fatal at boot.

### Onboarding a new property (operator workflow)

1. **Apply the Laravel patch** on the property's PMSApi (only once per
   property). See `laravel-patch/README.md`.
2. **Set on PMSApi `.env`:**
   - `KIOSK_DEVICE_KEY=<long random secret>`
   - `KIOSK_GROUP_ID=<the property's g_group.id>`
   - `KIOSK_LOOKUP_WINDOW_DAYS=2` (optional)
   - `KIOSK_DEVICE_ID=lobby-kiosk-01` (optional, audit-only label)
3. **Add a registry entry** to `go-proxy/properties.yaml` referencing a
   new env var for the same secret (e.g. `${ACME_DEVICE_KEY}`).
4. **Set the matching env** on the kiosk service (compose / k8s
   secret).
5. **Optional:** add a theme manifest in `src/theme/themes/<id>.ts` and
   register it in `src/theme/index.ts` if the property needs distinct
   visuals. Otherwise it falls back to `smart-moov`.
6. **Drop assets** at `public/themes/<id>/{hero.jpg,logo.svg}`.
7. **Restart the proxy.** No SPA rebuild required for steps 1–4.
8. Open `https://checkin.<your-domain>.com/<slug>` and verify.

---

## API surface

All routes under `/api/kiosk/v1/`. Only six legacy endpoints are
forwardable; everything else 404s. JSON in / JSON out.

| Method | Path                                      | Auth          | Purpose |
| ------ | ----------------------------------------- | ------------- | ------- |
| GET    | `/health`                                 | none          | liveness for readiness probes |
| GET    | `/api/kiosk/v1/ready`                     | none          | readiness — reports loaded property count |
| GET    | `/api/kiosk/v1/properties`                | none          | list all properties (sanitised) for an admin landing |
| GET    | `/api/kiosk/v1/{slug}/config`             | slug exists   | sanitised config for the active property |
| POST   | `/api/kiosk/v1/{slug}/lookup`             | slug exists   | → legacy `/api/kiosk/lookup` |
| POST   | `/api/kiosk/v1/{slug}/select`             | slug exists   | → legacy `/api/kiosk/select` |
| POST   | `/api/kiosk/v1/{slug}/form`               | slug exists   | → legacy `/api/kiosk/form` |
| POST   | `/api/kiosk/v1/{slug}/save-guest`         | slug exists   | → legacy `/api/kiosk/save-guest` |
| POST   | `/api/kiosk/v1/{slug}/save-firm`          | slug exists   | → legacy `/api/kiosk/save-firm` |
| POST   | `/api/kiosk/v1/{slug}/submit`             | slug exists   | → legacy `/api/kiosk/submit` |

### `/config` response (sanitized — safe for browser)

```json
{
  "slug": "smart-moov",
  "name": "smart moov",
  "subtitle": "hotel",
  "theme": "smart-moov",
  "languages": ["en", "de", "it"],
  "heroImage": "/themes/smart-moov/hero.jpg",
  "logo": "/themes/smart-moov/logo.svg",
  "supportPhone": "+39 0421 1830 350",
  "supportEmail": "reception@smart-moov-hotel.com"
}
```

Notice what's **not** there: `device_key`, `pmsapi_url`. Those stay
server-side.

### `/lookup` request shapes

```json
{ "lastName": "Rossi" }
{ "lastName": "Rossi", "arrivalDate": "2026-04-26" }
{ "reservationId": "BK-9911" }
```

Three outcomes:

```json
// 1. Single match
{ "result": "matched", "reservation": { "id": 12345, "code": "BK-9911", ... } }

// 2. Multiple matches — first-name picker
{
  "result": "ambiguous",
  "candidateToken": "<short-lived signed token>",
  "candidates": [{ "candidateId": "a", "firstName": "Mario" }, ...]
}

// 3. No match
{ "result": "not_found" }
```

The candidate token is signed (HMAC) and 2-min single-use.

---

## Data flow per check-in

Mews-style progressive lookup, three-step form.

```
Welcome
   │ tap "Begin check-in"
   ▼
Last-name input (Stage A)
   │ POST /lookup {lastName}
   │
   ├─ matched   → /checkin/1
   ├─ ambiguous → first-name picker (Stage B)
   │              │ POST /select {candidateToken, candidateId}
   │              └─→ matched → /checkin/1
   └─ not_found → "Please see reception"
                   ├─ "Try again" → Stage A
                   └─ "I have my booking number" → manual lookup

Step 1 — Guests
   │ for each guest:
   │   POST /save-guest {reservationId, guestIndex, guest}
   │
Step 2 — Firm/Extras
   │ POST /save-firm {reservationId, firm}
   │
Step 3 — Review + Sign
   │ POST /save-firm  (signature update)
   │ POST /submit {reservationId, lookupMethod, language, deviceId}
   │
   ▼
Done (auto-return after 6s)
```

### What the legacy side writes per submit

Mirrors `PreStayController` semantics — no migration needed.

```sql
-- Audit ping
INSERT INTO prestay_history
  (id_reservation, status, type, sequence, visited_at, ...)
  VALUES (?, 2 /* visited */, 'kiosk', 1, NOW(), ...);

-- Final flag flip — feeds Feratel pickup downstream
UPDATE room_reservation SET prestay = 1, updated_at = NOW() WHERE id = ?;
```

Per-guest data is in `room_reservation_guest`, firm + signature in
`room_reservation_firm` — both written incrementally by the per-step
saves, not the final submit.

### Idle reset

`IdleResetGuard.tsx` watches pointer/key events. After **90 s** of
inactivity it shows a 10-s countdown banner and then routes back to
`/`, clearing the session store. Partial DB writes from earlier steps
are not rolled back — that matches legacy behaviour where additive
guest entries persist across abandoned sessions.

---

## Theme system

Each theme is a TypeScript manifest under `src/theme/themes/`:

```ts
// src/theme/themes/smart-moov.ts
export const smartMoovTheme: ThemeManifest = {
  id: 'smart-moov',
  serifDisplay: false,
  fonts: { display: 'Geist, system-ui, sans-serif', body: 'Geist, system-ui, sans-serif' },
  palette: {
    bg: 'oklch(0.985 0 0)',
    ink: 'oklch(0.18 0.01 280)',
    brand: 'oklch(0.32 0.02 280)',
    accent: 'oklch(0.62 0.22 25)',          // signature red
    accentInk: '#ffffff',
    // ...
  },
};
```

`applyTheme(theme)` writes the palette + font as `--kt-*` CSS variables
on `:root`, which Tailwind v4 picks up via `@theme` in `index.css`.
Every component then uses utility classes like `bg-bg`, `text-ink`,
`bg-accent` — no theme-specific code in components.

**Adding a property's brand:**

1. Copy `src/theme/themes/smart-moov.ts` → `src/theme/themes/<id>.ts`,
   tune the palette + fonts.
2. Register in `src/theme/index.ts` (one line in the `themes` map).
3. Drop hero + logo at `public/themes/<id>/`.
4. Set `theme: <id>` in `properties.yaml`.

Two themes ship today:

| Theme | Visual direction | Reference |
| --- | --- | --- |
| `smart-moov` (default) | Modern urban — charcoal/white, bold red accent, sans-only | smart moov hotel |
| `pareus` | Refined-coastal — sand/limestone, adriatic blue, sun-gold, serif display | Pareus Beach Resort |

---

## Internationalisation

- `i18next` + `i18next-http-backend`.
- Bundles in `public/locales/{en,de,it,fr}/common.json`.
- Per-property language allowlist comes from the `/config` response so
  the language switcher can hide languages a property doesn't support.
- Numbers/dates use `Intl.*` with `i18n.resolvedLanguage`.

Keys are flat (no nested namespace). Adding a new language is two
files: drop a new `<lang>/common.json` and add the code to the
property's `languages: []` array.

---

## Security model

### Threat model

| Threat | Mitigation |
| --- | --- |
| Random web user hits `/api/kiosk/v1/.../lookup` | Per-property device key required by the legacy PMSApi; proxy adds it server-side. Browser can't bypass. |
| Stolen kiosk URL leaks property internals | `/config` returns only display fields. No PMSApi URL, no device key. |
| Compromised kiosk page (XSS) | Device key isn't in the browser at all. Worst case: attacker can call the proxy as if they were a kiosk — limited to the six allowlisted endpoints, scoped to one property. |
| Open relay abuse | Proxy allowlist (`internal/proxy/proxy.go::Allowed`). Only six legacy paths can be forwarded. |
| Tenant escape (property A reads property B) | `KIOSK_GROUP_ID` on each PMSApi pins the kiosk to one `g_group.id`. The proxy can only reach a property's own data because each property's `device_key` is paired with the matching `KIOSK_GROUP_ID` on its PMSApi. |
| Replay of `candidateToken` | Signed (HMAC), `exp` = 2 min, single-use enforced by handler. |
| Stale session after guest walks away | 90 s idle reset clears the in-progress form. |
| Signature bloat | Request body capped at 512 KB at the proxy; signature column-stored as PNG data URI. |

### What's intentionally NOT protected

- The **slug enumeration** is allowed: anyone can list properties via
  `/api/kiosk/v1/properties`. Slugs aren't secret.
- Last-name enumeration via `/lookup` is rate-limit-able on the legacy
  side but not blocked. This matches the Mews-style flow the project
  asked for.

### What lives where

| Secret | Location | Lifetime |
| --- | --- | --- |
| `<PROP>_DEVICE_KEY` env var | host / k8s Secret on the kiosk service | rotate manually |
| `KIOSK_DEVICE_KEY` on PMSApi | each property's `.env` | matches the kiosk-side value |
| `candidateToken` HMAC secret | derived from device key (or `KIOSK_LOOKUP_SECRET` if set) on PMSApi | per-property |

Rotating a property's device key:

1. Generate new random hex.
2. Update PMSApi `.env`, `php artisan config:clear`.
3. Update the kiosk's env var (compose/k8s).
4. Restart the kiosk service. ~30 s downtime for that one property.

---

## Legacy PMSApi patch

Lives at `laravel-patch/payload/`. Three artefacts land in the legacy
PMSApi tree; **no migrations** are run.

| File | Purpose |
| --- | --- |
| `app/Http/Middleware/KioskDeviceKey.php` | Validates `X-Device-Key` against env (constant-time compare) |
| `app/Http/Controllers/API/KioskController.php` | All six kiosk endpoints — lookup, select, form, save-guest, save-firm, submit |
| `app/Http/Kernel.php` | One-line edit registering `'kiosk.device'` middleware alias |
| `routes/api.php` | Route group `/api/kiosk/*` behind `kiosk.device` middleware |

Persistence (intentionally schema-free):

| Step | Table | Operation |
| --- | --- | --- |
| save-guest | `room_reservation_guest` | INSERT or UPDATE by `id` |
| save-firm | `room_reservation_firm` | INSERT or UPDATE by `reservation_id` |
| submit | `prestay_history` | INSERT one row, `type='kiosk'` |
| submit | `room_reservation` | UPDATE `prestay = 1` |

The legacy controller's `getDefaultConfig()` is mirrored in
`KioskController::defaultConfig()` so form rendering rules — `use`,
`required` (0/1/2) — match whatever the existing PreStay flow does.

---

## Deployment

### docker-compose (root of repo)

```bash
cp go-proxy/properties.example.yaml go-proxy/properties.yaml
# Edit properties.yaml to point at real PMSApi backends.

# Set the per-property device keys
export SMART_MOOV_DEVICE_KEY="$(openssl rand -hex 32)"
# (set also on each property's PMSApi .env as KIOSK_DEVICE_KEY)

docker compose up --build
# SPA at http://localhost:8080/<slug>
# Proxy at http://localhost:8089
```

### Kubernetes (sketch)

`go-proxy/k8s/deployment-dev.yaml` is a minimal Deployment + Service
manifest. Production needs:

- ConfigMap for `properties.yaml`.
- Secret per property's device key, mounted as env vars.
- An Ingress with a single hostname (`checkin.maiccube.com`) routing
  `/api/*` to the proxy and everything else to the SPA service.
- HPA optional — workload is light, single replica suffices for ~5
  properties.

### Required env on the kiosk host

| Var | Required | Purpose |
| --- | --- | --- |
| `PORT` | no (8089) | Go proxy port |
| `PROPERTIES_FILE` | no (`properties.yaml`) | path to registry |
| `ALLOWED_ORIGINS` | no (`*`) | CORS for the proxy |
| `UPSTREAM_TIMEOUT_MS` | no (12000) | per-request timeout to legacy |
| `<PROP>_DEVICE_KEY` | **yes per property** | referenced from YAML |

### Required env on each property's PMSApi

| Var | Required | Purpose |
| --- | --- | --- |
| `KIOSK_DEVICE_KEY` | **yes** | Must match `<PROP>_DEVICE_KEY` |
| `KIOSK_GROUP_ID` | **yes** | Property's `g_group.id`. Tenant pin. |
| `KIOSK_LOOKUP_WINDOW_DAYS` | no (2) | ± window around today for last-name lookup |
| `KIOSK_DEVICE_ID` | no | Label written to audit rows |

---

## Verification

### Manual smoke (on the kiosk host)

```bash
# 1. Health
curl http://localhost:8089/health
# → {"status":"healthy","service":"checkin-kiosk-api"}

# 2. Properties registry loaded
curl http://localhost:8089/api/kiosk/v1/properties

# 3. Property config (sanitized)
curl http://localhost:8089/api/kiosk/v1/smart-moov/config
# → {"slug":"smart-moov","name":"smart moov", ...}

# 4. End-to-end lookup
curl -X POST http://localhost:8089/api/kiosk/v1/smart-moov/lookup \
  -H 'Content-Type: application/json' \
  -d '{"lastName":"Mustermann"}'
# Browser would chain into /select, /form, /save-guest, /save-firm, /submit.
```

### DB checks after a successful submit (run on the property's MariaDB)

```sql
SELECT id, prestay, updated_at FROM room_reservation WHERE id = ?;
-- prestay = 1

SELECT id, reservation_id, first_name, last_name, document_id, nationality
  FROM room_reservation_guest WHERE reservation_id = ? ORDER BY id;

SELECT id, name, vat, CHAR_LENGTH(signature) AS sig_len,
       LEFT(signature, 22) = 'data:image/png;base64,' AS is_png
  FROM room_reservation_firm WHERE reservation_id = ?;

SELECT id, id_reservation, type, status, visited_at
  FROM prestay_history WHERE id_reservation = ? AND type = 'kiosk'
  ORDER BY id DESC LIMIT 1;
```

### Build + type checks

```bash
# SPA
npm install
npm run build              # Vite production build
npx tsc --noEmit           # strict type-check

# Go proxy
cd go-proxy
go build ./...
go vet ./...
```

---

## Open issues and follow-ups

1. **Production 500 from `/api/kiosk/lookup`.** During the initial
   smoke test we got HTTP 500 with an empty body and no entry in
   `storage/logs/laravel.log`. Suggests a PHP-FPM-level fatal that
   doesn't make it to Laravel's logger. Next step: tail the FPM error
   log (Plesk logs/ directory), repro, fix.
2. **Bundle size.** The SPA chunks at ~560 KB / 178 KB gzipped. Vite
   warns. Adding route-level lazy loading for the check-in step pages
   would halve it.
3. **No automated tests.** Manual smoke only. Targets to add:
   - `internal/config` unit tests for env-var interpolation, slug
     validation, duplicate-slug detection.
   - `internal/proxy` test that the allowlist rejects unknown paths.
   - Playwright E2E that walks Welcome → Step 3 → Done with a stubbed
     proxy.
4. **No admin viewer for kiosk submissions.** The review skill
   (`react-islands-mode`) recommended a small AdminSPA widget showing
   recent kiosk check-ins from `prestay_history WHERE type='kiosk'`.
   Out of scope for this iteration; queue as a follow-up.
5. **MRZ scanning** is not wired. Manual document entry only. The
   legacy PMSApi already exposes `MRZ_READER_URL`. Adding a
   `/api/kiosk/v1/{slug}/mrz` proxy entry + a step-3 capture button is
   straightforward when wanted.
6. **Browser kiosk lockdown** is out of scope (Chrome `--kiosk`,
   watchdog, autostart, screensaver disabled, pinch disabled). Flag
   for ops, not the code.
7. **Multi-property roaming.** Today a kiosk is pinned to one
   property's URL. If a hotel chain wanted one shared device that
   chooses a property at boot, we'd add a `/select-property` landing
   that lists `/api/kiosk/v1/properties` and routes the user.

---

## Review checklist

A reviewer can step through these top-to-bottom. Each item references
the file/area to inspect.

### Code-level

- [ ] **Property registry validation** — `go-proxy/internal/config/config.go`
  - Does the slug regex match what the SPA puts in URLs? (`slugRe`)
  - Are duplicate slugs rejected? (`Load()`)
  - Does `${VAR}` interpolation refuse to start when an env var is
    missing? (`expandEnvVars` + `validate`)
- [ ] **Proxy allowlist** — `go-proxy/internal/proxy/proxy.go`
  - Is `Allowed` exactly the six legacy paths, no more?
  - Are inbound headers stripped except for the small allowlist
    (`Content-Type`, `Accept-Language`, `X-Lookup-Method`,
    `X-Kiosk-Language`, `X-Forwarded-For`)?
  - Is the upstream body size capped (512 KB)?
- [ ] **Slug routing on the SPA** — `src/api/client.ts`
  - Does `propertySlug()` correctly handle reserved top-level paths?
  - Does the axios baseURL fail closed when no slug is present?
- [ ] **Property gate on boot** — `src/components/PropertyProvider.tsx`
  - Does the SPA refuse to render Welcome before `/config` resolves?
- [ ] **No build-time secrets** — search the SPA for `VITE_DEVICE_API_KEY`,
  `VITE_KIOSK_THEME`. Both should be gone.
- [ ] **Theme fallback** — `src/theme/index.ts::resolveTheme`
  - Does an unknown theme name fall back gracefully (no crash)?
- [ ] **Idle reset** — `src/components/IdleResetGuard.tsx`
  - 90 s threshold, 10 s countdown, clears session + form stores?

### Legacy-side

- [ ] **No migrations introduced** — `laravel-patch/` has no
  `database/migrations/` files.
- [ ] **`KioskController::submit`** writes only to existing tables.
  - `room_reservation.prestay = 1` flipped only here?
  - `prestay_history.type = 'kiosk'` row inserted?
- [ ] **`KioskDeviceKey`** uses constant-time compare (`hash_equals`).
- [ ] **Tenant pin** — `reservationBelongsToKiosk()` enforces
  `KIOSK_GROUP_ID` on every mutating call.

### Operational

- [ ] **Env documented** — `README.md` and this doc list every required
  env var.
- [ ] **Boot fails fast** — try starting the proxy with an unset
  `${SMART_MOOV_DEVICE_KEY}`; it should refuse to boot.
- [ ] **CORS allowlist** — production deploy sets `ALLOWED_ORIGINS` to
  the actual kiosk hostname, not `*`.
- [ ] **TLS** — kiosk hosted under HTTPS in production (single cert
  covers all properties since they share a domain).
- [ ] **Health checks** — `/health` and `/api/kiosk/v1/ready` wired to
  liveness + readiness probes.

### Security

- [ ] Browser bundle does **not** contain any device key (search the
  built `dist/` for known secret prefixes).
- [ ] `/config` response does **not** include `device_key` or
  `pmsapi_url`.
- [ ] Candidate token (`signLookup` / `verifyLookup` in
  `KioskController.php`) is HMAC-signed with a 2-min expiry.
- [ ] Per-property device key rotation is documented and tested.

### Design

- [ ] Visual parity check — does the kiosk's three-step flow render the
  same field set as the legacy `PreStayController` PreStayPage flow?
- [ ] Tap targets ≥ 56 px on tablet viewport (1024×768 landscape).
- [ ] Hero image lazy-falls-back when missing.
- [ ] Wordmark renders even with no logo asset (text-only fallback).

---

## Appendix: change history

| Date | Change |
| --- | --- |
| 2026-04-26 | Initial single-property kiosk + Laravel patch |
| 2026-04-26 | Themable per property (smart-moov, pareus) |
| 2026-04-27 | Multi-tenant refactor: one SPA + Go proxy, path-based property routing, YAML registry |
