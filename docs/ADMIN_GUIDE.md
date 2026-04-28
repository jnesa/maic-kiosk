# Admin Guide — MAIC Kiosk

Operator-facing runbook. Covers signing into the admin panel, registering
hotels, creating kiosk URLs, and the few maintenance actions you'll do
day-to-day.

If you're looking for the architecture / API contracts, read
[`SERVICE_DESIGN_v2.md`](./SERVICE_DESIGN_v2.md).
For backups / disaster recovery / deploys, read
[`OPERATIONS.md`](./OPERATIONS.md).

---

## Daily flow

| Task | Where |
| --- | --- |
| Sign in | `https://checkin.maiccube.com/admin/login` |
| Add a new hotel | **Hotels & kiosks** → **+ Add hotel** |
| Add a kiosk URL | open the hotel → **+ Add kiosk** |
| See all your kiosks | **Hotels & kiosks** → click a hotel |
| Audit who-did-what | **Audit log** in the left sidebar |

---

## Sign in

1. Navigate to the admin URL on the kiosk's domain. For a deployment at
   `checkin.maiccube.com`, that's `https://checkin.maiccube.com/admin`.
2. The panel redirects you to `/admin/login`.
3. Sign in with the email + password an admin set up via the CLI (see
   *Adding a new operator*).
4. Sessions last 12 hours and slide on activity. Sign out from the
   bottom-left of the sidebar when you're done.

If you forget your password, ask another admin to reset it via:

```bash
docker compose exec checkin /app/admin reset-password --email you@maiccube.com
```

You'll be prompted twice for the new password; all your active sessions
are revoked so you must sign in again.

---

## Onboarding a new property — full walkthrough

### Step 1 · prepare the legacy PMSApi

Once per property, before you touch the admin panel:

1. Apply the [Laravel patch](../laravel-patch/README.md) to the
   property's PMSApi (the `KioskController` + `KioskDeviceKey` middleware
   + the `/api/kiosk/*` route group).
2. Note the property's `g_group.id` on that PMSApi. You'll type this
   into the admin form. If the subdomain doesn't use groups (single
   tenant), leave it blank later.

### Step 2 · register the hotel

1. **Hotels & kiosks** → **+ Add hotel**.
2. Fill in:
   - **Display name** — what shows in this dashboard, e.g. `smart moov`.
   - **PMSApi URL** — the legacy backend's base URL,
     e.g. `https://pms.maiccube.com`. The kiosk forwards every guest
     request to `<this-url>/api/kiosk/<endpoint>`.
   - **Notes** (optional) — anything future-you will appreciate, e.g.
     "single tenant, group_id 72".
3. **Create hotel.**

### Step 3 · create a kiosk URL

1. Open the hotel from the list.
2. **+ Add kiosk**. Fill in:
   - **Display name** — e.g. `smart moov — lobby tablet`.
   - **Legacy group id** — the `g_group.id` from Step 1. Leave blank for
     single-tenant.
   - **Group label** — pretty name shown next to the id in this dashboard.
   - **Theme** — bundled theme to apply to the kiosk SPA. `smart-moov`
     and `pareus` ship today.
   - **Languages** — toggle the language flags the kiosk should offer.
   - *(Optional)* Hero image / logo URLs and support contact details.
3. **Create kiosk.** A modal opens with the new kiosk URL, a QR code,
   and the device key.

### Step 4 · configure the legacy `.env`

The admin panel shows you exactly what to paste:

```
KIOSK_DEVICE_KEY=<long random hex>
KIOSK_GROUP_ID=<the legacy group id you typed>
```

Set both on the property's PMSApi `.env`, then run:

```bash
php artisan config:clear
```

(On Plesk hosts where `php` isn't on PATH, delete the cached config
file directly: `rm -f bootstrap/cache/config.php`.)

### Step 5 · hand the URL to the property

- Copy the kiosk URL from the detail card.
- Email or print it for the property's tablet device. The QR code is
  intentionally large enough to scan from across the lobby.

### Step 6 · test once

Open the kiosk URL on any device. You should see the welcome screen
themed for the property. The first guest lookup verifies the legacy
side is correctly configured.

---

## Maintenance

### Rotate a device key

Used when a key has leaked or as a periodic precaution.

1. Open the hotel → click **Details** on the kiosk row.
2. **Rotate key**. The new key shows immediately; the old one stops
   working that instant.
3. **Update the property's PMSApi `.env`** with the new
   `KIOSK_DEVICE_KEY`, then `php artisan config:clear`. Until you do,
   the kiosk will return 401 for every guest action.

There's a confirmation dialog in the panel that reminds you about the
PMSApi side.

### Disable a kiosk temporarily

Useful while a property's tablet is in maintenance.

1. Kiosk detail → **Disable kiosk**.
2. The kiosk URL now returns 410 with a "deactivated" message. The
   guest sees a "Please see reception" screen.
3. Re-enable when ready: **Re-enable kiosk**.

No legacy-side change needed for this — the proxy just stops forwarding.

### Delete a kiosk

Hard delete. Kiosk URL returns 404 from then on. Audit-logged.

1. Kiosk detail → **Delete kiosk**, confirm.
2. Optionally remove `KIOSK_DEVICE_KEY` and `KIOSK_GROUP_ID` from the
   property's PMSApi `.env` for cleanliness.

### Delete a hotel

Cascades to all its kiosks. Same guidance as above for the legacy side.

### Update hotel details

Open the hotel detail, edit the PMSApi URL or notes inline. Useful if
a property migrates to a new domain.

---

## Adding a new operator

There's no public "register" surface — operators are provisioned by
another operator (or whoever has shell access to the host) using the
CLI bundled with the same Docker image.

```bash
docker compose exec checkin /app/admin add-user \
  --email luca@maiccube.com --name "Luca Rossi"
# Prompts for a password (typed twice, hidden). The user can sign in
# immediately after the command exits.
```

To list everyone:

```bash
docker compose exec checkin /app/admin list-users
```

To disable an operator (kills active sessions):

```bash
docker compose exec checkin /app/admin disable-user --email luca@maiccube.com
```

Re-enable later with `enable-user`.

---

## Audit log

Every state-changing action lands in the **Audit log** page:

- `login`, `logout`
- `create_hotel`, `update_hotel`, `delete_hotel`
- `create_kiosk`, `update_kiosk`, `rotate_key`, `disable_kiosk`,
  `enable_kiosk`, `delete_kiosk`
- `create_user`, `reset_password`, `disable_user`, `enable_user`
  (CLI-driven; actor shows as `cli:<unix user>`)

Sensitive payloads (device keys, password hashes) are stripped before
they're logged — you can show this page to internal compliance reviews
without redaction.

---

## What lives where (quick reference)

| Thing | Lives in |
| --- | --- |
| Hotel + kiosk records | SQLite at `/data/data.db` inside the container |
| Operator accounts | Same SQLite |
| Sessions | Same SQLite (cookie token → user) |
| Audit log | Same SQLite |
| Property device keys | SQLite — each kiosk row carries its own |
| Theme palettes / fonts | Bundled into the kiosk SPA at build time |
| Hero images / logos | Anywhere reachable by URL (the SPA loads them as `<img>`) |
| Legacy reservations / guests | The property's PMSApi (we never store these) |

---

## What you DON'T do here

- The kiosk panel can't talk to a property's database directly. If a
  reservation looks wrong, the property's PMSApi is the source of truth.
- The kiosk panel doesn't manage themes. Adding a new brand is a code
  change in `kiosk-spa/src/theme/themes/` — not a config tweak.
- The kiosk panel doesn't store guest data. Everything a guest types
  goes straight to the legacy PMSApi via the proxy.

If you need any of those, talk to engineering.
