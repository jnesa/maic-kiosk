<?php

namespace App\Http\Middleware;

use Closure;
use Illuminate\Http\Request;

/**
 * Authenticates a kiosk device using a shared secret in `X-Device-Key`.
 *
 * The expected key lives in env `KIOSK_DEVICE_KEY`. We use hash_equals for
 * a constant-time compare so that timing attacks can't be used to brute-force
 * the secret one byte at a time.
 *
 * Intentionally tenant-agnostic: kiosks are pinned to a single property via
 * env `KIOSK_GROUP_ID` on the kiosk side (and we re-check that in the
 * controller), so this middleware doesn't need access to the broken
 * `tenant(...)` helper that the legacy /api/login flow depends on.
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

        return $next($request);
    }
}
