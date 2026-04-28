# Laravel patch — `/api/kiosk/*` for legacy PMSApi

This patch adds a tenant-pinned, device-key-authenticated set of endpoints to
the live Laravel app at `pms.maiccube.com`.

**Persistence model — no new tables.**

| Where | What |
| --- | --- |
| `room_reservation_guest`        | Per-guest data (existing) |
| `room_reservation_firm`         | Firm/extras + signature (existing) |
| `prestay_history`               | Audit ping with `type='kiosk'` (existing) |
| `room_reservation.prestay = 1`  | Final flag (existing) |

## Files in this patch

```
payload/app/Http/Middleware/KioskDeviceKey.php
payload/app/Http/Controllers/API/KioskController.php
```

Plus two in-place edits to existing files:
- `app/Http/Kernel.php` — registers `'kiosk.device'` middleware alias
- `routes/api.php` — adds `/api/kiosk/*` route group

## Install

In the Plesk SSH terminal at the Laravel project root (the directory
containing `artisan`):

```bash
# 1. Place the new files
cp laravel-patch/payload/app/Http/Middleware/KioskDeviceKey.php           app/Http/Middleware/
cp laravel-patch/payload/app/Http/Controllers/API/KioskController.php   app/Http/Controllers/API/

# 2. Register middleware alias (manual edit if `sed` isn't available)
#    Open app/Http/Kernel.php and add this line inside $routeMiddleware,
#    right after the 'jwt.admin' line:
#
#        'kiosk.device' => \App\Http\Middleware\KioskDeviceKey::class,

# 3. Register routes (append to routes/api.php)
cat >> routes/api.php <<'PHP'

// Self check-in kiosk (tenant-pinned via env KIOSK_GROUP_ID, auth via X-Device-Key)
Route::middleware('kiosk.device')->prefix('kiosk')->group(function () {
    Route::post('lookup',     'API\KioskController@lookup');
    Route::post('select',     'API\KioskController@select');
    Route::post('form',       'API\KioskController@form');
    Route::post('save-guest', 'API\KioskController@saveGuest');
    Route::post('save-firm',  'API\KioskController@saveFirm');
    Route::post('submit',     'API\KioskController@submit');
});
PHP

# 4. Clear caches (the bootstrap/cache files; Plesk's CageFS often hides `php`)
rm -f bootstrap/cache/config.php bootstrap/cache/routes-v7.php \
      bootstrap/cache/services.php bootstrap/cache/packages.php
```

## Configure

Add to `.env`:

```
KIOSK_DEVICE_KEY=<long random secret — `openssl rand -hex 32`>
KIOSK_GROUP_ID=<smart-moov property's g_group.id>
KIOSK_LOOKUP_WINDOW_DAYS=2
KIOSK_DEVICE_ID=lobby-kiosk-01
```

## Smoke test

```bash
curl -i -X POST https://pms.maiccube.com/api/kiosk/lookup \
  -H 'Content-Type: application/json' -H 'Accept: application/json' \
  -H "X-Device-Key: $KIOSK_DEVICE_KEY" \
  -d '{"lastName":"Mustermann"}'
```

Expected outcomes:
- `{"result":"matched", "reservation": {...}}`
- `{"result":"ambiguous", "candidateToken": "...", "candidates": [...]}`
- `{"result":"not_found"}`

If you see `500` with an empty body, check `storage/logs/laravel.log`.

## Endpoint reference

| Method | Path | Body | Returns |
| --- | --- | --- | --- |
| POST | `/api/kiosk/lookup`     | `{lastName}` or `{reservationId}` or `{lastName, arrivalDate}` | matched / ambiguous / not_found |
| POST | `/api/kiosk/select`    | `{candidateToken, candidateId}` | matched reservation |
| POST | `/api/kiosk/form`      | `{reservationId}` | `{config, guests, firm, submitted, reservation}` |
| POST | `/api/kiosk/save-guest`| `{reservationId, guestIndex, guest}` | `{guestId}` |
| POST | `/api/kiosk/save-firm` | `{reservationId, firm}` | `{success}` |
| POST | `/api/kiosk/submit`    | `{reservationId, lookupMethod?, language?, deviceId?}` | flips `prestay=1`, writes prestay_history |

All require header `X-Device-Key: $KIOSK_DEVICE_KEY`.
