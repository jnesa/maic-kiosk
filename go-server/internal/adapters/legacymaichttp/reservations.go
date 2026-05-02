package legacymaichttp

import (
	"context"
	"net/http"

	"github.com/maic/checkin-kiosk-api/internal/domain"
)

// Search forwards to POST /api/kiosk_search_reservations {uuid, search}.
// Returns the legacy `reservations` array unchanged. The legacy
// controller filters by kiosk.group_id and `departure > today`.
func (c *Client) Search(ctx context.Context, kioskUUID, search string) ([]domain.ReservationSummary, error) {
	body := map[string]any{"uuid": kioskUUID, "search": search}
	var resp struct {
		Reservations []domain.ReservationSummary `json:"reservations"`
	}
	if err := c.postJSON(ctx, "/kiosk_search_reservations", body, &resp, nil); err != nil {
		return nil, err
	}
	return resp.Reservations, nil
}

// PrestayForm forwards to POST /api/kiosk_prestay_form {uuid}. The
// `prestay_form` payload is opaque; we return it as a generic map for
// the SPA to render.
func (c *Client) PrestayForm(ctx context.Context, kioskUUID string) (domain.PrestayConfig, error) {
	body := map[string]any{"uuid": kioskUUID}
	var resp struct {
		PrestayForm domain.PrestayConfig `json:"prestay_form"`
	}
	if err := c.postJSON(ctx, "/kiosk_prestay_form", body, &resp, nil); err != nil {
		return nil, err
	}
	if resp.PrestayForm == nil {
		// Empty form is technically valid — return an empty map so
		// callers don't have to nil-check.
		return domain.PrestayConfig{}, nil
	}
	return resp.PrestayForm, nil
}

// SaveGuest forwards to POST /api/kiosk_save_guest {reservation_id, guest}.
// The legacy route delegates to PreStayController::saveGuest, which
// upserts into room_reservation_guest.
//
// Returns the upserted guest_id. Legacy response shape varies (some
// builds return {id}, others {guest_id}, some return the full guest);
// we accept either.
func (c *Client) SaveGuest(ctx context.Context, kioskUUID, reservationID string, guest domain.GuestData, op *domain.Operator) (int64, error) {
	body := map[string]any{
		"reservation_id": reservationID,
		"guest":          guest,
		"uuid":           kioskUUID, // belt-and-braces — legacy ignores if unused
	}
	headers := optHeaders(op)

	var resp map[string]any
	if err := c.postJSON(ctx, "/kiosk_save_guest", body, &resp, headers); err != nil {
		return 0, err
	}
	if id := pickInt64(resp, "guest_id", "id"); id != 0 {
		return id, nil
	}
	if g, ok := resp["guest"].(map[string]any); ok {
		return pickInt64(g, "id"), nil
	}
	// Some builds just acknowledge with status:1 and no id. Fall
	// back to whatever the SPA sent (could be 0 for new guests).
	if guest.ID != nil {
		return *guest.ID, nil
	}
	return 0, nil
}

// SaveFirm forwards to POST /api/kiosk_save_firm {reservation_id,
// ...flat firm fields}. The Postman collection shows the legacy
// controller expects firm fields at the top level, NOT inside a
// `firm:{}` envelope. We spread the FirmData here.
func (c *Client) SaveFirm(ctx context.Context, kioskUUID, reservationID string, firm domain.FirmData, op *domain.Operator) error {
	body := flattenFirm(firm)
	body["reservation_id"] = reservationID
	body["uuid"] = kioskUUID
	headers := optHeaders(op)
	return c.postJSON(ctx, "/kiosk_save_firm", body, nil, headers)
}

// DeleteGuest forwards to POST /api/kiosk_prestay_delete_guest
// {reservation_id, guest_id}. Soft-delete; the legacy controller
// flips a flag on room_reservation_guest.
func (c *Client) DeleteGuest(ctx context.Context, kioskUUID, reservationID string, guestID int64, op *domain.Operator) error {
	body := map[string]any{
		"reservation_id": reservationID,
		"guest_id":       guestID,
		"uuid":           kioskUUID,
	}
	headers := optHeaders(op)
	return c.postJSON(ctx, "/kiosk_prestay_delete_guest", body, nil, headers)
}

// SaveForm is the optional one-shot submit. The Postman collection
// has it but newDev's routes/api.php does NOT — open question §3.5.4 Q1
// in the audit doc.
//
// We attempt the call; if the upstream returns 404 (route missing),
// we surface ErrNotFound so the handler can fall back to multi-step
// SaveGuest+SaveFirm.
func (c *Client) SaveForm(ctx context.Context, kioskUUID, reservationID string, payload map[string]any, op *domain.Operator) error {
	if payload == nil {
		payload = map[string]any{}
	}
	payload["reservation_id"] = reservationID
	payload["uuid"] = kioskUUID
	headers := optHeaders(op)
	return c.postJSON(ctx, "/kiosk_save_form", payload, nil, headers)
}

// flattenFirm spreads a FirmData into the flat top-level fields the
// legacy `kiosk_save_firm` endpoint expects. Match the Postman example
// body: compname, vatid, address, city, arrival, arrival_via,
// arrival_with_car, phone, email, useFirmForBilling,
// useAnotherBillingAddress, billing_address, transfer, transferText,
// babyBed, babyBedText, dogPackage, dogPackageText, additionalLinens,
// additionalLinensAmount, alergies, alergiesText, accessible,
// preferedCommunication, sign.
//
// Mapping is direct; only `signature` (SPA) → `sign` (legacy) needs
// renaming.
func flattenFirm(f domain.FirmData) map[string]any {
	return map[string]any{
		"compname":                  f.CompName,
		"vatid":                     f.VatID,
		"address":                   f.Address,
		"city":                      f.City,
		"arrival":                   f.Arrival,
		"arrival_via":               f.ArrivalVia,
		"arrival_with_car":          f.ArrivalWithCar,
		"phone":                     f.Phone,
		"email":                     f.Email,
		"useFirmForBilling":         f.UseFirmForBilling,
		"useAnotherBillingAddress":  f.UseAnotherBillingAddr,
		"billing_address":           f.BillingAddress,
		"transfer":                  f.Transfer,
		"transferText":              f.TransferText,
		"babyBed":                   f.BabyBed,
		"babyBedText":               f.BabyBedText,
		"dogPackage":                f.DogPackage,
		"dogPackageText":            f.DogPackageText,
		"additionalLinens":          f.AdditionalLinens,
		"additionalLinensAmount":    f.AdditionalLinensCount,
		"alergies":                  f.Allergies,
		"alergiesText":              f.AllergiesText,
		"accessible":                f.Accessible,
		"preferedCommunication":     f.PreferredCommunication,
		"sign":                      f.Signature,
	}
}

func optHeaders(op *domain.Operator) http.Header {
	if op == nil {
		return nil
	}
	return operatorHeaders(*op)
}
