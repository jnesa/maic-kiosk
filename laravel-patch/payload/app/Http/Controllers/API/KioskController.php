<?php

namespace App\Http\Controllers\API;

use App\AppSettingGroup;
use App\Http\Controllers\Controller;
use App\PrestayHistory;
use App\Room;
use App\RoomReservation;
use App\RoomReservationFirm;
use App\RoomReservationGuest;
use App\RoomReservationRoom;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Facades\Log;
use Illuminate\Support\Facades\Schema;
// DB is still used by the wrapping transaction in submit().

/**
 * Self-Check-In Kiosk endpoints.
 *
 * These routes are tenant-pinned via env (`KIOSK_GROUP_ID`) so the controller
 * intentionally avoids the legacy `tenant(...)` helper that crashes outside
 * a known hostname tenant. Auth is by `X-Device-Key` (KioskDeviceKey middleware).
 *
 * Persistence model — NO new tables. We reuse the legacy schema exactly the
 * way the existing PreStayController does:
 *   - Per-guest data        → room_reservation_guest
 *   - Firm/extras + signature → room_reservation_firm
 *   - Audit ping            → prestay_history (existing shape, type='kiosk')
 *   - Final flag            → room_reservation.prestay = 1
 *
 * Endpoints:
 *   POST /api/kiosk/lookup            — progressive lookup (last name | reservation code)
 *   POST /api/kiosk/select            — pick one candidate from /lookup ambiguous
 *   POST /api/kiosk/form              — form config + prefill (per reservation)
 *   POST /api/kiosk/save-guest        — upsert one guest
 *   POST /api/kiosk/save-firm         — upsert firm + signature
 *   POST /api/kiosk/submit            — finalize: prestay=1 + prestay_history audit row
 */
class KioskController extends Controller
{
    /**
     * Window of days around today to consider for lookup. Configurable via
     * `KIOSK_LOOKUP_WINDOW_DAYS` so a property with very early arrivals can
     * widen it without code changes.
     */
    private function lookupWindow(): int
    {
        return (int) env('KIOSK_LOOKUP_WINDOW_DAYS', 2);
    }

    private function kioskGroupId(): int
    {
        return (int) env('KIOSK_GROUP_ID', 0);
    }

    /**
     * Default prestay form configuration. Mirrors
     * PMSApi PreStayController::getDefaultConfig() so any rendering logic the
     * legacy SPA relies on continues to work.
     */
    private function defaultConfig(): array
    {
        return [
            'f_name'              => ['use' => true,  'required' => 1],
            'l_name'              => ['use' => true,  'required' => 1],
            'dob'                 => ['use' => true,  'required' => 1],
            'document'            => ['use' => true,  'required' => 1],
            'document_id'         => ['use' => true,  'required' => 1],
            'country'             => ['use' => true,  'required' => 1],
            'nationality'         => ['use' => true,  'required' => 1],
            'street'              => ['use' => true,  'required' => 0],
            'city'                => ['use' => true,  'required' => 0],
            'postal'              => ['use' => true,  'required' => 0],
            'phone'               => ['use' => false, 'required' => 0],
            'title'               => ['use' => false, 'required' => 0],
            'document_issuer'     => ['use' => false, 'required' => 0],
            'document_issue_date' => ['use' => false, 'required' => 0],
            'signature-pad'       => ['use' => true,  'required' => 0],
            'traveltime'          => ['use' => false, 'required' => 0],
            'businesstravel'      => ['use' => false, 'required' => 0],
            'annualcard'          => ['use' => false, 'required' => 0],
            'handicap'            => ['use' => false, 'required' => 0],
            'transfer'            => ['use' => false, 'required' => 0],
            'babyBed'             => ['use' => false, 'required' => 0],
            'dogPackage'          => ['use' => false, 'required' => 0],
            'alergies'            => ['use' => false, 'required' => 0],
        ];
    }

    /** Builds the JSON-friendly summary the kiosk SPA renders on the welcome card. */
    private function reservationSummary(RoomReservation $r): array
    {
        // Try to find the assigned room for display.
        $roomName = '';
        $groupId  = 0;
        $rrr = $r->room_reservation_room()->first();
        if ($rrr) {
            $room = Room::find($rrr->room_id);
            if ($room) {
                $roomName = (string) $room->room_no;
                $groupId  = (int) ($room->id_group ?? 0);
            }
        }

        // The new Postgres orchestrator schema dropped the legacy
        // `room_reservation.prestay` flag. Completion is now derived from
        // `prestay_history`: a row with status=2 (visited) means the guest
        // already finished the prestay/check-in flow.
        $prestayDone = PrestayHistory::where('id_reservation', (int) $r->id)
            ->where('status', 2)
            ->exists();

        return [
            'id'           => (int) $r->id,
            'code'         => (string) ($r->reservation_id ?? ''),
            'firstName'    => (string) ($r->first_name ?? ''),
            'lastName'     => (string) ($r->last_name ?? ''),
            'arrival'      => (string) ($r->arrival ?? ''),
            'departure'    => (string) ($r->departure ?? ''),
            'adults'       => (int) ($r->adults ?? 1),
            'children'     => (int) ($r->children ?? 0),
            'roomName'     => $roomName,
            'groupId'      => $groupId,
            'prestayDone'  => $prestayDone,
        ];
    }

    /**
     * POST /api/kiosk/lookup
     *
     * Three accepted body shapes:
     *   { "lastName": "Rossi" }
     *   { "reservationId": "BK-9911" }
     *   { "lastName": "Rossi", "arrivalDate": "YYYY-MM-DD" }
     *
     * Outcomes:
     *   { "result": "matched",   "reservation": {...} }
     *   { "result": "ambiguous", "candidates": [{candidateId, firstName}, ...], "lookupId": "<opaque>" }
     *   { "result": "not_found" }
     */
    public function lookup(Request $request): JsonResponse
    {
        $groupId = $this->kioskGroupId();
        if ($groupId <= 0) {
            return response()->json(['error' => ['code' => 'misconfigured', 'message' => 'KIOSK_GROUP_ID not set']], 500);
        }

        $lastName     = trim((string) $request->input('lastName', ''));
        $reservation  = trim((string) $request->input('reservationId', ''));
        $arrivalDate  = trim((string) $request->input('arrivalDate', ''));

        $base = RoomReservation::query()
            ->select(
                'room_reservation.id',
                'room_reservation.reservation_id',
                'room_reservation.first_name',
                'room_reservation.last_name',
                'room_reservation.arrival',
                'room_reservation.departure',
                'room_reservation.adults',
                'room_reservation.children'
            )
            ->leftJoin('room_reservation_room', 'room_reservation_room.reservation_id', '=', 'room_reservation.id')
            ->leftJoin('rooms', 'rooms.id', '=', 'room_reservation_room.room_id')
            ->where(function ($q) use ($groupId) {
                $q->where('rooms.id_group', $groupId)
                  ->orWhereNull('rooms.id_group');
            });

        if ($reservation !== '') {
            $r = $base->clone()->where('room_reservation.reservation_id', $reservation)->first();
            if (! $r) {
                return response()->json(['result' => 'not_found']);
            }
            return response()->json(['result' => 'matched', 'reservation' => $this->reservationSummary($r)]);
        }

        if ($lastName === '') {
            return response()->json(['error' => ['code' => 'bad_request', 'message' => 'lastName or reservationId required']], 400);
        }

        $base = $base
            ->whereRaw('LOWER(room_reservation.last_name) = LOWER(?)', [$lastName]);

        if ($arrivalDate !== '') {
            $base = $base->whereDate('room_reservation.arrival', $arrivalDate);
        } else {
            // Window expressed as concrete dates so the SQL stays portable
            // across MariaDB (legacy) and Postgres (new orchestrator). PHP
            // computes the bounds; the DB just sees ISO date strings.
            $win = $this->lookupWindow();
            $now = now();
            $base = $base->whereBetween('room_reservation.arrival', [
                $now->copy()->subDays($win)->toDateString(),
                $now->copy()->addDays($win)->toDateString(),
            ]);
        }

        $hits = $base->orderBy('room_reservation.arrival')->limit(25)->get();

        if ($hits->count() === 0) {
            return response()->json(['result' => 'not_found']);
        }
        if ($hits->count() === 1) {
            return response()->json(['result' => 'matched', 'reservation' => $this->reservationSummary($hits->first())]);
        }

        // Ambiguous — return masked candidates. We pack the resolved row ids
        // into a short-lived signed payload so the /select endpoint can trust
        // the chosen candidateId without a separate session store.
        $candidates = [];
        $idMap      = [];
        foreach ($hits as $i => $r) {
            $cid = chr(ord('a') + ($i % 26)) . ($i >= 26 ? (string) intdiv($i, 26) : '');
            $candidates[] = ['candidateId' => $cid, 'firstName' => (string) $r->first_name];
            $idMap[$cid]  = (int) $r->id;
        }
        $payload   = ['ids' => $idMap, 'exp' => now()->addMinutes(2)->timestamp, 'gid' => $groupId];
        $lookupTok = $this->signLookup($payload);

        return response()->json([
            'result'         => 'ambiguous',
            'candidateToken' => $lookupTok,
            'candidates'     => $candidates,
        ]);
    }

    /**
     * POST /api/kiosk/select
     * Body: { "candidateToken": "...", "candidateId": "a" }
     */
    public function select(Request $request): JsonResponse
    {
        $tok = (string) $request->input('candidateToken', '');
        $cid = (string) $request->input('candidateId', '');
        $payload = $this->verifyLookup($tok);
        if (! $payload) {
            return response()->json(['error' => ['code' => 'candidate_expired', 'message' => 'Candidate token expired']], 401);
        }
        if (((int) $payload['gid']) !== $this->kioskGroupId()) {
            return response()->json(['error' => ['code' => 'tenant_mismatch']], 403);
        }
        if (! isset($payload['ids'][$cid])) {
            return response()->json(['error' => ['code' => 'bad_candidate']], 400);
        }
        $r = RoomReservation::find($payload['ids'][$cid]);
        if (! $r) {
            return response()->json(['result' => 'not_found']);
        }
        return response()->json(['result' => 'matched', 'reservation' => $this->reservationSummary($r)]);
    }

    /**
     * POST /api/kiosk/form
     * Body: { "reservationId": <int> }
     * Returns: { config, guests[], firm|null, submitted, reservation }
     */
    public function form(Request $request): JsonResponse
    {
        $reservationId = (int) $request->input('reservationId');
        $r = RoomReservation::find($reservationId);
        if (! $r) {
            return response()->json(['error' => ['code' => 'not_found']], 404);
        }

        // Tenant pin — the kiosk should never see another property's data.
        $summary = $this->reservationSummary($r);
        if ($summary['groupId'] !== 0 && $summary['groupId'] !== $this->kioskGroupId()) {
            return response()->json(['error' => ['code' => 'tenant_mismatch']], 403);
        }

        // Merge defaults with per-group override JSON.
        $cfg = $this->defaultConfig();
        if ($summary['groupId'] !== 0) {
            $settingGroup = AppSettingGroup::whereHas('groups', function ($q) use ($summary) {
                $q->where('g_group.id', $summary['groupId']);
            })->first();
            if ($settingGroup && $settingGroup->prestay_form) {
                $override = json_decode($settingGroup->prestay_form, true);
                if (is_array($override)) {
                    $cfg = array_replace($cfg, $override);
                }
            }
        }

        $guests = RoomReservationGuest::where('reservation_id', $r->id)
            ->orderBy('id')
            ->get()
            ->map(function ($g) {
                return [
                    'id'                  => (int) $g->id,
                    'title'               => (string) ($g->title ?? ''),
                    'fname'               => (string) ($g->first_name ?? ''),
                    'lname'               => (string) ($g->last_name ?? ''),
                    'dob'                 => (string) ($g->dob ?? ''),
                    'country'             => (string) ($g->country ?? ''),
                    'city'                => (string) ($g->city ?? ''),
                    'postal'              => (string) ($g->postal ?? ''),
                    'street'              => (string) ($g->address ?? ''),
                    'house_number'        => (string) ($g->house_number ?? ''),
                    'document'            => $g->document_type !== null ? (int) $g->document_type : null,
                    'document_id'         => (string) ($g->document_id ?? ''),
                    'document_issuer'     => (string) ($g->document_issuer ?? ''),
                    'document_issue_date' => (string) ($g->document_issue_date ?? ''),
                    'nationality'         => (string) ($g->nationality ?? ''),
                    'phone'               => (string) ($g->phone ?? ''),
                    'annualcard'          => (bool) ($g->annualcard ?? false),
                    'annualcard_number'   => (string) ($g->annualcard_number ?? ''),
                ];
            })->all();

        $firmRow = RoomReservationFirm::where('reservation_id', $r->id)->first();
        $firm    = $firmRow ? [
            'compname'                  => (string) ($firmRow->name ?? ''),
            'vatid'                     => (string) ($firmRow->vat ?? ''),
            'address'                   => (string) ($firmRow->address ?? ''),
            'city'                      => (string) ($firmRow->city ?? ''),
            'arrival'                   => (string) ($firmRow->arrival ?? ''),
            'arrival_via'               => (string) ($firmRow->arrival_via ?? ''),
            'arrival_with_car'          => (bool) ($firmRow->arrival_with_car ?? false),
            'phone'                     => (string) ($firmRow->phone ?? ''),
            'email'                     => (string) ($firmRow->email ?? ''),
            'useFirmForBilling'         => (bool) ($firmRow->useFirmForBilling ?? false),
            'useAnotherBillingAddress'  => (bool) ($firmRow->useAnotherBillingAddress ?? false),
            'billing_address'           => (string) ($firmRow->billing_address ?? ''),
            'transfer'                  => (bool) ($firmRow->transfer ?? false),
            'transferText'              => (string) ($firmRow->transferText ?? ''),
            'babyBed'                   => (bool) ($firmRow->babyBed ?? false),
            'babyBedText'               => (string) ($firmRow->babyBedText ?? ''),
            'dogPackage'                => (bool) ($firmRow->dogPackage ?? false),
            'dogPackageText'            => (string) ($firmRow->dogPackageText ?? ''),
            'alergies'                  => (bool) ($firmRow->alergies ?? false),
            'alergiesText'              => (string) ($firmRow->alergiesText ?? ''),
            'accessible'                => (bool) ($firmRow->accessible ?? false),
            'additionalLinens'          => (bool) ($firmRow->additionLinens ?? false),
            'additionalLinensAmount'    => (string) ($firmRow->additionLinensAmount ?? ''),
            'preferedCommunication'     => (string) ($firmRow->preferedCommunication ?? ''),
            'signature'                 => (string) ($firmRow->signature ?? ''),
        ] : null;

        return response()->json([
            'config'      => $cfg,
            'guests'      => $guests,
            'firm'        => $firm,
            'submitted'   => $summary['prestayDone'],
            'reservation' => $summary,
        ]);
    }

    /**
     * POST /api/kiosk/save-guest
     * Body: { reservationId, guestIndex, guest: {...} }
     */
    public function saveGuest(Request $request): JsonResponse
    {
        $reservationId = (int) $request->input('reservationId');
        $guest = $request->input('guest', []);
        if (! is_array($guest)) {
            return response()->json(['error' => ['code' => 'bad_request']], 400);
        }
        if (! $this->reservationBelongsToKiosk($reservationId)) {
            return response()->json(['error' => ['code' => 'tenant_mismatch']], 403);
        }

        $row = isset($guest['id']) && (int) $guest['id'] > 0
            ? RoomReservationGuest::where('reservation_id', $reservationId)->where('id', (int) $guest['id'])->first()
            : null;

        if (! $row) {
            $row = new RoomReservationGuest();
            $row->reservation_id = $reservationId;
        }

        $row->title               = (string) ($guest['title'] ?? '');
        $row->first_name          = (string) ($guest['fname'] ?? '');
        $row->last_name           = (string) ($guest['lname'] ?? '');
        $row->dob                 = ($guest['dob'] ?? '') ?: null;
        $row->country             = (string) ($guest['country'] ?? '');
        $row->city                = (string) ($guest['city'] ?? '');
        $row->postal              = (string) ($guest['postal'] ?? '');
        $row->address             = (string) ($guest['street'] ?? '');
        $row->house_number        = (string) ($guest['house_number'] ?? '');
        $row->document_type       = isset($guest['document']) && $guest['document'] !== null ? (int) $guest['document'] : null;
        $row->document_id         = (string) ($guest['document_id'] ?? '');
        $row->document_issuer     = (string) ($guest['document_issuer'] ?? '');
        $row->document_issue_date = ($guest['document_issue_date'] ?? '') ?: null;
        $row->nationality         = (string) ($guest['nationality'] ?? '');
        $row->phone               = (string) ($guest['phone'] ?? '');
        $row->annualcard          = ! empty($guest['annualcard']) ? 1 : 0;
        $row->annualcard_number   = (string) ($guest['annualcard_number'] ?? '');
        $row->save();

        return response()->json(['success' => true, 'guestId' => (int) $row->id]);
    }

    /**
     * POST /api/kiosk/save-firm
     * Body: { reservationId, firm: {...} }
     */
    public function saveFirm(Request $request): JsonResponse
    {
        $reservationId = (int) $request->input('reservationId');
        $firm = $request->input('firm', []);
        if (! is_array($firm)) {
            return response()->json(['error' => ['code' => 'bad_request']], 400);
        }
        if (! $this->reservationBelongsToKiosk($reservationId)) {
            return response()->json(['error' => ['code' => 'tenant_mismatch']], 403);
        }

        $row = RoomReservationFirm::where('reservation_id', $reservationId)->first();
        if (! $row) {
            $row = new RoomReservationFirm();
            $row->reservation_id = $reservationId;
        }

        $row->name                     = (string) ($firm['compname'] ?? '');
        $row->vat                      = (string) ($firm['vatid'] ?? '');
        $row->address                  = (string) ($firm['address'] ?? '');
        $row->city                     = (string) ($firm['city'] ?? '');
        $row->arrival                  = (string) ($firm['arrival'] ?? '');
        $row->arrival_via              = (string) ($firm['arrival_via'] ?? '');
        $row->arrival_with_car         = ! empty($firm['arrival_with_car']) ? 1 : 0;
        $row->phone                    = (string) ($firm['phone'] ?? '');
        $row->email                    = (string) ($firm['email'] ?? '');
        $row->useFirmForBilling        = ! empty($firm['useFirmForBilling']) ? 1 : 0;
        $row->useAnotherBillingAddress = ! empty($firm['useAnotherBillingAddress']) ? 1 : 0;
        $row->billing_address          = (string) ($firm['billing_address'] ?? '');
        $row->transfer                 = ! empty($firm['transfer']) ? 1 : 0;
        $row->transferText             = (string) ($firm['transferText'] ?? '');
        $row->babyBed                  = ! empty($firm['babyBed']) ? 1 : 0;
        $row->babyBedText              = (string) ($firm['babyBedText'] ?? '');
        $row->dogPackage               = ! empty($firm['dogPackage']) ? 1 : 0;
        $row->dogPackageText           = (string) ($firm['dogPackageText'] ?? '');
        $row->alergies                 = ! empty($firm['alergies']) ? 1 : 0;
        $row->alergiesText             = (string) ($firm['alergiesText'] ?? '');
        $row->accessible               = ! empty($firm['accessible']) ? 1 : 0;
        $row->additionLinens           = ! empty($firm['additionalLinens']) ? 1 : 0;
        $row->additionLinensAmount     = (string) ($firm['additionalLinensAmount'] ?? '');
        $row->preferedCommunication    = (string) ($firm['preferedCommunication'] ?? '');
        $row->signature                = (string) ($firm['signature'] ?? '');
        $row->save();

        return response()->json(['success' => true]);
    }

    /**
     * POST /api/kiosk/submit
     * Body: { reservationId, lookupMethod, language, deviceId }
     *
     * No new tables — we reuse the existing legacy schema:
     *   - guest data was already saved in /save-guest (room_reservation_guest)
     *   - firm + signature was already saved in /save-firm (room_reservation_firm)
     * This endpoint writes a `prestay_history` audit row (type='kiosk',
     * status=2) which is the new orchestrator's source of truth for
     * "prestay/check-in complete" — the legacy `room_reservation.prestay`
     * column was dropped. We also set `check_in` so downstream consumers
     * (Feratel pickup, dashboards) see the timestamp.
     */
    public function submit(Request $request): JsonResponse
    {
        $reservationId = (int) $request->input('reservationId');
        if (! $this->reservationBelongsToKiosk($reservationId)) {
            return response()->json(['error' => ['code' => 'tenant_mismatch']], 403);
        }
        $r = RoomReservation::find($reservationId);
        if (! $r) {
            return response()->json(['error' => ['code' => 'not_found']], 404);
        }

        try {
            DB::transaction(function () use ($r, $reservationId) {
                // Audit ping using the existing prestay_history table — same
                // shape as any other "visited" event in the legacy app, only
                // the type is set to 'kiosk' so dashboards can distinguish it.
                PrestayHistory::create([
                    'id_reservation' => $reservationId,
                    'status'         => 2, // STATUS_VISITED
                    'type'           => 'kiosk',
                    'sequence'       => 1,
                    'visited_at'     => now(),
                ]);

                // The legacy `prestay` flag is gone; the prestay_history row
                // above is the new completion signal. Stamp `check_in` so
                // any downstream pipeline that watches that timestamp picks
                // the kiosk submission up.
                if (Schema::hasColumn('room_reservation', 'check_in')) {
                    $r->check_in = now();
                }
                $r->save();
            });
        } catch (\Throwable $e) {
            Log::error('kiosk submit failed', ['err' => $e->getMessage(), 'reservation_id' => $reservationId]);
            return response()->json(['error' => ['code' => 'submit_failed', 'message' => $e->getMessage()]], 502);
        }

        return response()->json([
            'success'       => true,
            'reservationId' => $reservationId,
            'checkedInAt'   => now()->toIso8601String(),
        ]);
    }

    /* ------------------------------------------------------------------ */
    /* Helpers                                                             */
    /* ------------------------------------------------------------------ */

    private function reservationBelongsToKiosk(int $reservationId): bool
    {
        $r = RoomReservation::find($reservationId);
        if (! $r) return false;
        $rrr = $r->room_reservation_room()->first();
        if (! $rrr) {
            // Reservation without a room assignment — accept it (early creation).
            return true;
        }
        $room = Room::find($rrr->room_id);
        if (! $room || ! $room->id_group) return true;
        return ((int) $room->id_group) === $this->kioskGroupId();
    }

    /**
     * Sign / verify a tiny lookup-candidate token. We use a base64+HMAC scheme
     * rather than a full JWT to avoid pulling in another dependency for this
     * one-purpose 2-minute payload.
     */
    private function signLookup(array $payload): string
    {
        $body = base64_encode(json_encode($payload));
        $sig  = hash_hmac('sha256', $body, $this->lookupSecret());
        return $body . '.' . $sig;
    }

    private function verifyLookup(string $token): ?array
    {
        $parts = explode('.', $token, 2);
        if (count($parts) !== 2) return null;
        [$body, $sig] = $parts;
        $expected = hash_hmac('sha256', $body, $this->lookupSecret());
        if (! hash_equals($expected, $sig)) return null;
        $payload = json_decode(base64_decode($body), true);
        if (! is_array($payload)) return null;
        if (($payload['exp'] ?? 0) < now()->timestamp) return null;
        return $payload;
    }

    /**
     * HMAC secret for lookup tokens. Falls back to the device key so a kiosk
     * deployment that only sets KIOSK_DEVICE_KEY still works. Operators can
     * set KIOSK_LOOKUP_SECRET separately to rotate it independently.
     */
    private function lookupSecret(): string
    {
        return (string) env('KIOSK_LOOKUP_SECRET', env('KIOSK_DEVICE_KEY', 'kiosk-default-secret'));
    }
}
