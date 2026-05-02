package legacymaichttp

import (
	"context"
	"net/http"

	"github.com/maic/checkin-kiosk-api/internal/domain"
)

// ListForUser implements ports.KioskStore.
//
// Forwards to POST /api/get_users_kiosks {user_id}. The legacy
// controller (NewMAIC origin/newDev:KioskController@get_users_kiosks)
// scopes by Administrator.id_broker → groups → Kiosk.group_id.
//
// We pass the operator's session token in the Authorization header
// when present — some legacy builds gate this route behind auth:api
// even though the dev environment doesn't.
func (c *Client) ListForUser(ctx context.Context, op domain.Operator) ([]domain.Kiosk, error) {
	body := map[string]any{"user_id": op.UserID}
	headers := operatorHeaders(op)

	var resp struct {
		Kiosks []domain.Kiosk `json:"kiosks"`
	}
	if err := c.postJSON(ctx, "/get_users_kiosks", body, &resp, headers); err != nil {
		return nil, err
	}
	return resp.Kiosks, nil
}

// GetByUUID resolves a kiosk by its public UUID by listing the
// operator's kiosks and filtering. The legacy app has no dedicated
// "get kiosk by uuid" endpoint — `kiosk_prestay_form` does its own
// lookup but doesn't return the row, so we reuse the operator listing.
//
// PHASE 1 LIMITATION: this requires an operator context. For
// guest-flow paths (which have no operator), the kiosk Go service
// caches the kiosk record in-memory after the operator creates it OR
// trusts the UUID and lets the upstream calls validate it.
//
// In practice the SPA calls /config first; the service derives a
// minimal config from the UUID alone and lets `kiosk_prestay_form`
// (which DOES validate) fail at the form-fetch step if the UUID is
// bogus.
func (c *Client) GetByUUID(ctx context.Context, uuid string) (*domain.Kiosk, error) {
	// Without an operator-scoped endpoint, the cheapest way to
	// validate a UUID is to ask for its prestay form — the legacy
	// controller looks up `Kiosk::where('uuid', $uuid)` and returns
	// `admin.error` if not found.
	//
	// We don't return the full row here (the prestay-form endpoint
	// doesn't echo it). The caller treats this as a liveness check.
	body := map[string]any{"uuid": uuid}
	var resp map[string]any
	if err := c.postJSON(ctx, "/kiosk_prestay_form", body, &resp, nil); err != nil {
		return nil, err
	}
	// Liveness confirmed; return a minimal Kiosk shape. The id and
	// group_id are not in the response.
	return &domain.Kiosk{UUID: uuid}, nil
}

// operatorHeaders adds the operator's legacy session token as a Bearer
// Authorization header. Returns nil if there's no token (so callers
// can pass nil-safely to postJSON).
func operatorHeaders(op domain.Operator) http.Header {
	if op.LegacySessionToken == "" {
		return nil
	}
	h := http.Header{}
	h.Set("Authorization", "Bearer "+op.LegacySessionToken)
	return h
}
