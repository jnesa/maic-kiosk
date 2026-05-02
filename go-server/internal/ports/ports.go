// Package ports defines the interfaces the kiosk service's handlers
// depend on. Adapters in internal/adapters/* implement these interfaces
// against concrete transports — today it's HTTP to the legacy MAIC
// monolith (legacymaichttp); tomorrow it could be Redis Streams or
// direct DB. Handler code never imports an adapter package directly.
package ports

import (
	"context"

	"github.com/maic/checkin-kiosk-api/internal/domain"
)

// OperatorAuth handles the operator-side login flow. The kiosk Go
// service forwards the operator's credentials to the legacy MAIC app
// and stores the returned session token in a cookie.
type OperatorAuth interface {
	// Login authenticates the operator against the legacy app's
	// /api/login endpoint and returns the user record + the legacy
	// session token captured from the response.
	Login(ctx context.Context, username, password, deviceID string) (*domain.LoginResult, error)
}

// KioskStore lists kiosks the operator can administer. Phase-1 reads
// this from the legacy `kiosks` table via /api/get_users_kiosks; phase-2
// could read from a Horizon service via Redis Streams or direct DB —
// same interface either way.
type KioskStore interface {
	// ListForUser returns every kiosk visible to the operator with
	// the given legacy user_id. The legacy adapter joins through
	// admin_users.id_broker → groups → kiosks.group_id.
	ListForUser(ctx context.Context, op domain.Operator) ([]domain.Kiosk, error)

	// GetByUUID resolves a kiosk by its public UUID. Used to derive
	// the public /config payload and to confirm the kiosk exists
	// before forwarding guest-flow operations.
	GetByUUID(ctx context.Context, uuid string) (*domain.Kiosk, error)
}

// Reservations is the guest-flow port: lookup, prestay form config,
// guest/firm save+delete, and the optional one-shot save_form. Every
// method takes the kiosk UUID so the adapter can pass it upstream and
// let the legacy app scope by group_id.
type Reservations interface {
	// Search looks up reservations matching the free-text `search`
	// term (last name, first name, or reservation_id) on the kiosk's
	// group, with departure > today.
	Search(ctx context.Context, kioskUUID, search string) ([]domain.ReservationSummary, error)

	// PrestayForm returns the merged prestay form config for the
	// kiosk's property — defaults + per-group overrides.
	PrestayForm(ctx context.Context, kioskUUID string) (domain.PrestayConfig, error)

	// SaveGuest upserts one guest into a reservation. Returns the
	// guest_id (newly assigned or existing).
	SaveGuest(ctx context.Context, kioskUUID, reservationID string, guest domain.GuestData, op *domain.Operator) (int64, error)

	// SaveFirm upserts the firm/billing block + signature.
	SaveFirm(ctx context.Context, kioskUUID, reservationID string, firm domain.FirmData, op *domain.Operator) error

	// DeleteGuest removes a guest from the reservation's prestay list.
	DeleteGuest(ctx context.Context, kioskUUID, reservationID string, guestID int64, op *domain.Operator) error

	// SaveForm is the one-shot submit if/when the legacy app exposes
	// /api/kiosk_save_form. Adapters that don't have it return
	// ErrNotImplemented; the handler then falls back to driving
	// SaveGuest × N + SaveFirm itself.
	SaveForm(ctx context.Context, kioskUUID, reservationID string, payload map[string]any, op *domain.Operator) error
}

// Feratel is the postal-code → city helper used during guest data
// entry. Backed by the legacy app's Feratel integration.
type Feratel interface {
	// FetchCityFromPostal queries the legacy /api/kiosk_fetch_city_from_postal
	// helper. Returns the raw upstream payload as a map so the SPA
	// (which already knows the legacy shape) can render it.
	FetchCityFromPostal(ctx context.Context, kioskUUID, reservationID, postal, country string) (map[string]any, error)
}
