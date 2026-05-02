package legacymaichttp_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/maic/checkin-kiosk-api/internal/adapters/legacymaichttp"
	"github.com/maic/checkin-kiosk-api/internal/domain"
)

// fakeLegacy spins up an httptest.Server that mimics the relevant
// subset of the legacy MAIC PHP monolith's behaviour.
type fakeLegacy struct {
	t          *testing.T
	server     *httptest.Server
	loginToken string
}

func newFakeLegacy(t *testing.T) *fakeLegacy {
	t.Helper()
	f := &fakeLegacy{t: t, loginToken: "test-token-123"}
	f.server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeLegacy) serve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/api/login":
		_ = r.ParseForm()
		if r.Form.Get("username") != "alice" || r.Form.Get("password") != "wonderland" {
			f.writeErr(w, "invalid credentials")
			return
		}
		f.write(w, map[string]any{
			"access_token": f.loginToken,
			"user":         map[string]any{"id": 7, "email": "alice@example.com", "name": "Alice"},
		})
	case "/api/get_users_kiosks":
		var b struct{ UserID int64 `json:"user_id"` }
		_ = json.NewDecoder(r.Body).Decode(&b)
		if b.UserID == 0 {
			f.writeErr(w, "user_id required")
			return
		}
		f.write(w, map[string]any{
			"kiosks": []map[string]any{
				{"id": 1, "name": "Lobby", "uuid": "kiosk-uuid-1", "group_id": 11},
				{"id": 2, "name": "Spa", "uuid": "kiosk-uuid-2", "group_id": 11},
			},
		})
	case "/api/kiosk_prestay_form":
		var b struct{ UUID string `json:"uuid"` }
		_ = json.NewDecoder(r.Body).Decode(&b)
		if b.UUID == "" {
			f.writeErr(w, "uuid required")
			return
		}
		if b.UUID == "unknown" {
			f.writeErr(w, "admin.error")
			return
		}
		f.write(w, map[string]any{
			"prestay_form": map[string]any{
				"useMRZ": true,
				"f_name": map[string]any{"use": true, "required": 1.0},
			},
		})
	case "/api/kiosk_search_reservations":
		var b struct {
			UUID   string `json:"uuid"`
			Search string `json:"search"`
		}
		_ = json.NewDecoder(r.Body).Decode(&b)
		if b.UUID == "" || b.Search == "" {
			f.writeErr(w, "missing params")
			return
		}
		f.write(w, map[string]any{
			"reservations": []map[string]any{
				{
					"reservation_id": "ABC-1",
					"first_name":     "Carmen",
					"last_name":      "Pieper",
					"arrival":        "2026-11-20",
					"departure":      "2026-11-23",
				},
			},
		})
	case "/api/kiosk_save_guest":
		var b struct {
			ReservationID string `json:"reservation_id"`
			Guest         map[string]any
		}
		_ = json.NewDecoder(r.Body).Decode(&b)
		if b.ReservationID == "" {
			f.writeErr(w, "reservation_id required")
			return
		}
		f.write(w, map[string]any{"id": 42})
	case "/api/kiosk_save_firm":
		var raw map[string]any
		_ = json.NewDecoder(r.Body).Decode(&raw)
		// Verify firm fields land at the top level (flat shape).
		if raw["compname"] == nil {
			f.writeErr(w, "compname required at top level")
			return
		}
		f.write(w, map[string]any{"ok": true})
	case "/api/kiosk_prestay_delete_guest":
		var b struct {
			ReservationID string `json:"reservation_id"`
			GuestID       int64  `json:"guest_id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&b)
		if b.ReservationID == "" || b.GuestID == 0 {
			f.writeErr(w, "missing")
			return
		}
		f.write(w, map[string]any{"ok": true})
	case "/api/kiosk_fetch_city_from_postal":
		var b struct {
			Postal  string `json:"postal"`
			Country string `json:"country"`
		}
		_ = json.NewDecoder(r.Body).Decode(&b)
		if b.Postal == "1010" && b.Country == "AT" {
			f.write(w, map[string]any{"success": true, "city": "Wien"})
			return
		}
		f.write(w, map[string]any{"success": false})
	case "/api/kiosk_save_form":
		// Legacy newDev doesn't have this — return 404 to verify
		// the adapter surfaces ErrNotFound.
		http.NotFound(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (f *fakeLegacy) write(w http.ResponseWriter, payload map[string]any) {
	if payload == nil {
		payload = map[string]any{}
	}
	payload["status"] = 1
	payload["status_message"] = ""
	_ = json.NewEncoder(w).Encode(payload)
}

func (f *fakeLegacy) writeErr(w http.ResponseWriter, msg string) {
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":         0,
		"status_message": msg,
	})
}

// --- tests ---

func TestLogin_Success(t *testing.T) {
	f := newFakeLegacy(t)
	c := legacymaichttp.New(f.server.URL, 5*time.Second)

	res, err := c.Login(context.Background(), "alice", "wonderland", "dev-1")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if res.Operator.LegacySessionToken != f.loginToken {
		t.Errorf("token = %q, want %q", res.Operator.LegacySessionToken, f.loginToken)
	}
	if res.Operator.UserID != 7 {
		t.Errorf("user_id = %d, want 7", res.Operator.UserID)
	}
	if res.Operator.Email != "alice@example.com" {
		t.Errorf("email = %q", res.Operator.Email)
	}
}

func TestLogin_BadCreds_ReturnsUpstream(t *testing.T) {
	// Laravel BaseController::sendError returns HTTP 200 with status:0;
	// our client maps that to ErrUpstream.
	f := newFakeLegacy(t)
	c := legacymaichttp.New(f.server.URL, 5*time.Second)

	_, err := c.Login(context.Background(), "alice", "wrong", "")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, domain.ErrUpstream) {
		t.Errorf("error = %v, want ErrUpstream", err)
	}
}

func TestListForUser(t *testing.T) {
	f := newFakeLegacy(t)
	c := legacymaichttp.New(f.server.URL, 5*time.Second)

	kiosks, err := c.ListForUser(context.Background(), domain.Operator{UserID: 7})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(kiosks) != 2 {
		t.Fatalf("got %d kiosks, want 2", len(kiosks))
	}
	if kiosks[0].UUID != "kiosk-uuid-1" || kiosks[0].GroupID != 11 {
		t.Errorf("first kiosk = %+v", kiosks[0])
	}
}

func TestListForUser_MissingUserID(t *testing.T) {
	f := newFakeLegacy(t)
	c := legacymaichttp.New(f.server.URL, 5*time.Second)

	_, err := c.ListForUser(context.Background(), domain.Operator{})
	if err == nil {
		t.Fatal("expected error for missing user_id")
	}
}

func TestPrestayForm(t *testing.T) {
	f := newFakeLegacy(t)
	c := legacymaichttp.New(f.server.URL, 5*time.Second)

	cfg, err := c.PrestayForm(context.Background(), "kiosk-uuid-1")
	if err != nil {
		t.Fatalf("form: %v", err)
	}
	if cfg["useMRZ"] != true {
		t.Errorf("useMRZ = %v, want true", cfg["useMRZ"])
	}
}

func TestPrestayForm_UnknownUUID(t *testing.T) {
	f := newFakeLegacy(t)
	c := legacymaichttp.New(f.server.URL, 5*time.Second)

	_, err := c.PrestayForm(context.Background(), "unknown")
	if err == nil {
		t.Fatal("expected error for unknown UUID")
	}
	if !errors.Is(err, domain.ErrUpstream) {
		t.Errorf("error = %v, want ErrUpstream", err)
	}
}

func TestSearch(t *testing.T) {
	f := newFakeLegacy(t)
	c := legacymaichttp.New(f.server.URL, 5*time.Second)

	rs, err := c.Search(context.Background(), "kiosk-uuid-1", "Pieper")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(rs) != 1 {
		t.Fatalf("got %d results, want 1", len(rs))
	}
	if rs[0].ReservationID != "ABC-1" || rs[0].LastName != "Pieper" {
		t.Errorf("result = %+v", rs[0])
	}
}

func TestSaveGuest(t *testing.T) {
	f := newFakeLegacy(t)
	c := legacymaichttp.New(f.server.URL, 5*time.Second)

	id, err := c.SaveGuest(context.Background(), "kiosk-uuid-1", "ABC-1", domain.GuestData{FName: "X", LName: "Y"}, nil)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if id != 42 {
		t.Errorf("id = %d, want 42", id)
	}
}

func TestSaveFirm_FlattensFields(t *testing.T) {
	// Verify the adapter spreads firm fields to the top level rather
	// than nesting under `firm:{}` (legacy expects flat shape).
	f := newFakeLegacy(t)
	c := legacymaichttp.New(f.server.URL, 5*time.Second)

	err := c.SaveFirm(context.Background(), "kiosk-uuid-1", "ABC-1",
		domain.FirmData{CompName: "Acme", VatID: "ATU123", Signature: "data:image/png;base64,..."},
		nil,
	)
	if err != nil {
		t.Fatalf("save firm: %v", err)
	}
}

func TestDeleteGuest(t *testing.T) {
	f := newFakeLegacy(t)
	c := legacymaichttp.New(f.server.URL, 5*time.Second)

	if err := c.DeleteGuest(context.Background(), "kiosk-uuid-1", "ABC-1", 42, nil); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestSaveForm_RouteMissing_ReturnsNotFound(t *testing.T) {
	// Legacy newDev doesn't have this route; verify ErrNotFound.
	f := newFakeLegacy(t)
	c := legacymaichttp.New(f.server.URL, 5*time.Second)

	err := c.SaveForm(context.Background(), "kiosk-uuid-1", "ABC-1", map[string]any{}, nil)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

func TestFetchCityFromPostal_Hit(t *testing.T) {
	f := newFakeLegacy(t)
	c := legacymaichttp.New(f.server.URL, 5*time.Second)

	out, err := c.FetchCityFromPostal(context.Background(), "kiosk-uuid-1", "ABC-1", "1010", "AT")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if out["success"] != true {
		t.Errorf("success = %v", out["success"])
	}
	if out["city"] != "Wien" {
		t.Errorf("city = %v", out["city"])
	}
}

func TestFetchCityFromPostal_Miss(t *testing.T) {
	f := newFakeLegacy(t)
	c := legacymaichttp.New(f.server.URL, 5*time.Second)

	out, err := c.FetchCityFromPostal(context.Background(), "kiosk-uuid-1", "ABC-1", "99999", "XX")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if out["success"] != false {
		t.Errorf("success = %v, want false", out["success"])
	}
}

func TestOperatorTokenForwarded(t *testing.T) {
	// When an operator is supplied, the adapter should attach a
	// Bearer Authorization header to outbound calls.
	var captured string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"kiosks":         []any{},
			"status":         1,
			"status_message": "",
		})
	}))
	defer srv.Close()

	c := legacymaichttp.New(srv.URL, 5*time.Second)
	op := domain.Operator{UserID: 7, LegacySessionToken: "t-xyz"}
	_, _ = c.ListForUser(context.Background(), op)

	want := "Bearer t-xyz"
	if captured != want {
		t.Errorf("Authorization = %q, want %q", captured, want)
	}
}

func TestNetworkFailure_WrapsAsUpstream(t *testing.T) {
	// Point at a closed port to force a connection error; verify the
	// adapter wraps it as ErrUpstream so handlers can map it.
	c := legacymaichttp.New("http://127.0.0.1:1", 200*time.Millisecond)
	_, err := c.PrestayForm(context.Background(), "anything")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, domain.ErrUpstream) {
		t.Errorf("error = %v, want ErrUpstream wrap", err)
	}
}

// Smoke test that postForm encodes correctly.
func TestPostForm_EncodesFields(t *testing.T) {
	var got url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		got = r.Form
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":   "tok",
			"user":           map[string]any{"id": 1},
			"status":         1,
			"status_message": "",
		})
	}))
	defer srv.Close()

	c := legacymaichttp.New(srv.URL, 5*time.Second)
	_, err := c.Login(context.Background(), "u", "p", "d")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if got.Get("action") != "login" {
		t.Errorf("action = %q", got.Get("action"))
	}
	if got.Get("device_id") != "d" {
		t.Errorf("device_id = %q", got.Get("device_id"))
	}
}

// Ensure the adapter sends a content-type the legacy stack expects.
func TestPostJSON_SendsJSONContentType(t *testing.T) {
	var ct string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ct = r.Header.Get("Content-Type")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"reservations":   []any{},
			"status":         1,
			"status_message": "",
		})
	}))
	defer srv.Close()

	c := legacymaichttp.New(srv.URL, 5*time.Second)
	_, _ = c.Search(context.Background(), "u", "x")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}
