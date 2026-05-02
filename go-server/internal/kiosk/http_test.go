package kiosk_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/maic/checkin-kiosk-api/internal/adapters/fakes"
	"github.com/maic/checkin-kiosk-api/internal/kiosk"
)

func newKioskServer() (http.Handler, *fakes.Reservations, *fakes.Feratel, *fakes.KioskStore) {
	res := fakes.NewReservations()
	feratel := fakes.NewFeratel()
	ks := fakes.NewKioskStore()
	h := kiosk.New(res, feratel, ks)

	r := chi.NewRouter()
	r.Route("/api/kiosk/v1", h.Mount)
	return r, res, feratel, ks
}

func doKiosk(t *testing.T, h http.Handler, method, path string, body any) *http.Response {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		buf, _ := json.Marshal(body)
		rdr = bytes.NewReader(buf)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w.Result()
}

func decodeBody(t *testing.T, r *http.Response) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.NewDecoder(r.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return out
}

func TestHealth(t *testing.T) {
	srv, _, _, _ := newKioskServer()
	resp := doKiosk(t, srv, http.MethodGet, "/api/kiosk/v1/health", nil)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	body := decodeBody(t, resp)
	if body["status"] != "healthy" {
		t.Errorf("status = %v", body["status"])
	}
}

func TestConfig_KnownUUID(t *testing.T) {
	srv, _, _, _ := newKioskServer()
	resp := doKiosk(t, srv, http.MethodGet, "/api/kiosk/v1/kiosk-uuid-1/config", nil)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	body := decodeBody(t, resp)
	if body["uuid"] != "kiosk-uuid-1" {
		t.Errorf("uuid = %v", body["uuid"])
	}
}

func TestConfig_UnknownUUID_Returns410(t *testing.T) {
	srv, _, _, _ := newKioskServer()
	resp := doKiosk(t, srv, http.MethodGet, "/api/kiosk/v1/unknown/config", nil)
	if resp.StatusCode != http.StatusGone {
		t.Errorf("status = %d, want 410", resp.StatusCode)
	}
}

func TestLookup_OneMatch_ReturnsMatched(t *testing.T) {
	srv, _, _, _ := newKioskServer()
	resp := doKiosk(t, srv, http.MethodPost, "/api/kiosk/v1/kiosk-uuid-1/lookup",
		map[string]string{"search": "Pieper"})
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	body := decodeBody(t, resp)
	if body["result"] != "matched" {
		t.Errorf("result = %v, want 'matched'", body["result"])
	}
	if body["token"] != "ABC-1" {
		t.Errorf("token = %v", body["token"])
	}
	res, _ := body["reservation"].(map[string]any)
	if res["code"] != "ABC-1" || res["lastName"] != "Pieper" {
		t.Errorf("reservation = %+v", res)
	}
}

func TestLookup_NoMatch_ReturnsNotFound(t *testing.T) {
	srv, _, _, _ := newKioskServer()
	resp := doKiosk(t, srv, http.MethodPost, "/api/kiosk/v1/kiosk-uuid-1/lookup",
		map[string]string{"search": "Nobody"})
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	body := decodeBody(t, resp)
	if body["result"] != "not_found" {
		t.Errorf("result = %v, want 'not_found'", body["result"])
	}
}

func TestLookup_AcceptsLastNameField(t *testing.T) {
	// SPA may send `lastName` instead of `search`.
	srv, _, _, _ := newKioskServer()
	resp := doKiosk(t, srv, http.MethodPost, "/api/kiosk/v1/kiosk-uuid-1/lookup",
		map[string]string{"lastName": "Pieper"})
	if resp.StatusCode != 200 {
		t.Errorf("status = %d", resp.StatusCode)
	}
}

func TestLookup_MissingSearch_Returns400(t *testing.T) {
	srv, _, _, _ := newKioskServer()
	resp := doKiosk(t, srv, http.MethodPost, "/api/kiosk/v1/kiosk-uuid-1/lookup",
		map[string]string{})
	if resp.StatusCode != 400 {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestSelect_ReturnsMatched(t *testing.T) {
	srv, res, _, _ := newKioskServer()
	// Seed the search with a reservation_id-keyed result so /select
	// can re-fetch the full row.
	res.SearchResults["ABC-1"] = res.SearchResults["Pieper"]

	resp := doKiosk(t, srv, http.MethodPost, "/api/kiosk/v1/kiosk-uuid-1/select",
		map[string]string{"candidateId": "ABC-1"})
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	body := decodeBody(t, resp)
	if body["result"] != "matched" {
		t.Errorf("result = %v", body["result"])
	}
	r, _ := body["reservation"].(map[string]any)
	if r["code"] != "ABC-1" {
		t.Errorf("reservation = %+v", r)
	}
}

func TestForm_ReturnsConfig(t *testing.T) {
	srv, _, _, _ := newKioskServer()
	resp := doKiosk(t, srv, http.MethodPost, "/api/kiosk/v1/kiosk-uuid-1/form",
		map[string]string{"reservationId": "ABC-1"})
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	body := decodeBody(t, resp)
	form, _ := body["prestay_form"].(map[string]any)
	if form["useMRZ"] != true {
		t.Errorf("useMRZ = %v", form["useMRZ"])
	}
}

func TestSaveGuest_ReturnsID(t *testing.T) {
	srv, res, _, _ := newKioskServer()
	resp := doKiosk(t, srv, http.MethodPost, "/api/kiosk/v1/kiosk-uuid-1/save-guest", map[string]any{
		"reservationId": "ABC-1",
		"guest":         map[string]any{"fname": "X", "lname": "Y"},
	})
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	body := decodeBody(t, resp)
	if id, _ := body["id"].(float64); id != 100 {
		t.Errorf("id = %v, want 100", body["id"])
	}
	if len(res.SaveGuestCalls) != 1 {
		t.Fatalf("expected 1 save call, got %d", len(res.SaveGuestCalls))
	}
	call := res.SaveGuestCalls[0]
	if call.UUID != "kiosk-uuid-1" || call.ReservationID != "ABC-1" || call.Guest.FName != "X" {
		t.Errorf("call = %+v", call)
	}
}

func TestSaveFirm_DelegatesToPort(t *testing.T) {
	srv, res, _, _ := newKioskServer()
	resp := doKiosk(t, srv, http.MethodPost, "/api/kiosk/v1/kiosk-uuid-1/save-firm", map[string]any{
		"reservationId": "ABC-1",
		"firm":          map[string]any{"compname": "Acme", "vatid": "V123"},
	})
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if len(res.SaveFirmCalls) != 1 {
		t.Fatalf("expected 1 firm call, got %d", len(res.SaveFirmCalls))
	}
	if res.SaveFirmCalls[0].Firm.CompName != "Acme" {
		t.Errorf("firm = %+v", res.SaveFirmCalls[0].Firm)
	}
}

func TestDeleteGuest_DelegatesToPort(t *testing.T) {
	srv, res, _, _ := newKioskServer()
	resp := doKiosk(t, srv, http.MethodPost, "/api/kiosk/v1/kiosk-uuid-1/delete-guest", map[string]any{
		"reservationId": "ABC-1",
		"guestId":       42,
	})
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if len(res.DeleteGuestCalls) != 1 {
		t.Fatalf("expected 1 delete call, got %d", len(res.DeleteGuestCalls))
	}
	if res.DeleteGuestCalls[0].GuestID != 42 {
		t.Errorf("guest_id = %d", res.DeleteGuestCalls[0].GuestID)
	}
}

func TestSubmit_NoOpAck(t *testing.T) {
	srv, _, _, _ := newKioskServer()
	resp := doKiosk(t, srv, http.MethodPost, "/api/kiosk/v1/kiosk-uuid-1/submit", map[string]string{})
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	body := decodeBody(t, resp)
	if body["ok"] != true {
		t.Errorf("ok = %v", body["ok"])
	}
}

func TestPostalLookup_Hit(t *testing.T) {
	srv, _, _, _ := newKioskServer()
	resp := doKiosk(t, srv, http.MethodPost, "/api/kiosk/v1/kiosk-uuid-1/postal-lookup",
		map[string]string{"postal": "1010", "country": "AT"})
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	body := decodeBody(t, resp)
	if body["success"] != true {
		t.Errorf("success = %v", body["success"])
	}
}

func TestPostalLookup_MissingPostal_400(t *testing.T) {
	srv, _, _, _ := newKioskServer()
	resp := doKiosk(t, srv, http.MethodPost, "/api/kiosk/v1/kiosk-uuid-1/postal-lookup",
		map[string]string{})
	if resp.StatusCode != 400 {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}
