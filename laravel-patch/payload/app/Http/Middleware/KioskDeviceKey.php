<?php

namespace App\Http\Middleware;

use Closure;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\DB;

/**
 * Authenticates a kiosk device using a shared secret in `X-Device-Key`.
 *
 * The expected key lives in env `KIOSK_DEVICE_KEY`. We use hash_equals for
 * a constant-time compare so that timing attacks can't be used to brute-force
 * the secret one byte at a time.
 *
 * After authentication we pin the request to KIOSK_GROUP_ID by hooking into
 * the TenantContext service. That sets the Postgres `app.tenant_id` session
 * variable so RLS policies allow the kiosk to read its own tenant's rows.
 * Without this step every kiosk query would return zero rows under RLS.
 */
class KioskDeviceKey
{
    public function handle(Request $request, Closure $next)
    {
        $expected = (string) env('KIOSK_DEVICE_KEY', '');
        $provided = (string) $request->header('X-Device-Key', '');

        if ($expected === '' || ! hash_equals($expected, $provided)) {
            return response()->json([
                'error' => [
                    'code'    => 'device_unauthorized',
                    'message' => 'Invalid kiosk device key',
                ],
            ], 401);
        }

        // Set Postgres RLS context. Prefer the explicit KIOSK_TENANT_ID env
        // because `shared.g_group` itself has RLS — looking up tenant_id
        // there requires already having a tenant set, which is the
        // chicken-and-egg we are trying to escape. The kiosk is pinned to
        // a single property, so the operator is expected to declare both
        // KIOSK_GROUP_ID and KIOSK_TENANT_ID.
        $kioskTenantId = (int) env('KIOSK_TENANT_ID', 0);
        if ($kioskTenantId > 0) {
            try {
                DB::statement("SET app.tenant_id = '{$kioskTenantId}'");
            } catch (\Throwable $e) {
                \Log::warning('KIOSK tenant init failed: ' . $e->getMessage());
            }
        }

        return $next($request);
    }
}
