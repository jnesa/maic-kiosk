package main_test

// Integration test: spins up the real handlers + real legacymaichttp
// adapter pointed at an httptest.Server that mocks the legacy MAIC
// PHP endpoints. Drives the same wire shapes the SPA would, end to
// end. Failure here means a real change broke something the SPA
// relies on.

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/maic/checkin-kiosk-api/internal/adapters/legacymaichttp"
	"github.com/maic/checkin-kiosk-api/internal/admin"
	"github.com/maic/checkin-kiosk-api/internal/kiosk"
	"github.com/maic/checkin-kiosk-api/internal/session"
)

const integSecret = "0123456789abcdef0123456789abcdef"

// fakeLegacyServer mimics the live `https://dev.maiccube.com` API for
// the routes we actually call.
func fakeLegacyServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/login", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		w.Header().Set("Content-Type", "application/json")
		if r.Form.Get("username") != "alice" || r.Form.Get("password") != "wonderland" {
			_ = json.NewEncoder(w).Encode(map[string]any{"status": 0, "status_message": "invalid"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "live-token",
			"user":         map[string]any{"id": 7, "name": "Alice", "email": "alice@example.com"},
			"status":       1, "status_message": "",
		})
	})
	mux.HandleFunc("/api/get_users_kiosks", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"kiosks": []map[string]any{
				{"id": 1, "name": "Lobby", "uuid": "uuid-1", "group_id": 11},
			},
			"status": 1, "status_message": "",
		})
	})
	mux.HandleFunc("/api/kiosk_prestay_form", func(w http.ResponseWriter, r *http.Request) {
		var b struct{ UUID string `json:"uuid"` }
		_ = json.NewDecoder(r.Body).Decode(&b)
		w.Header().Set("Content-Type", "application/json")
		if b.UUID != "uuid-1" {
			_ = json.NewEncoder(w).Encode(map[string]any{"status": 0, "status_message": "admin.error"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"prestay_form": map[string]any{"useMRZ": true},
			"status":       1, "status_message": "",
		})
	})
	mux.HandleFunc("/api/kiosk_search_reservations", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"reservations": []map[string]any{
				{"reservation_id": "ABC-1", "first_name": "Carmen", "last_name": "Pieper", "arrival": "2026-11-20", "departure": "2026-11-23"},
			},
			"status": 1, "status_message": "",
		})
	})
	mux.HandleFunc("/api/kiosk_save_guest", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 42, "status": 1, "status_message": ""})
	})
	mux.HandleFunc("/api/kiosk_save_firm", func(w http.ResponseWriter, r *http.Request) {
		var raw map[string]any
		_ = json.NewDecoder(r.Body).Decode(&raw)
		// Verify the firm fields land flat at the top level (not nested).
		if raw["compname"] == nil || raw["vatid"] == nil {
			t.Errorf("firm fields missing at top level: %+v", raw)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"status": 1, "status_message": ""})
	})
	mux.HandleFunc("/api/kiosk_prestay_delete_guest", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"status": 1, "status_message": ""})
	})
	mux.HandleFunc("/api/kiosk_fetch_city_from_postal", func(w http.ResponseWriter, r *http.Request) {
		var b struct{ Postal string `json:"postal"` }
		_ = json.NewDecoder(r.Body).Decode(&b)
		w.Header().Set("Content-Type", "application/json")
		if b.Postal == "1010" {
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "city": "Wien", "status": 1})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "status": 1})
	})

	return httptest.NewServer(mux)
}

func buildKioskServer(t *testing.T, legacyURL string) http.Handler {
	t.Helper()
	upstream := legacymaichttp.New(legacyURL, 5*time.Second)
	sm := session.NewManager(integSecret)
	adminH := admin.New(upstream, upstream, sm)
	kioskH := kiosk.New(upstream, upstream, upstream)

	r := chi.NewRouter()
	r.Use(sm.Middleware)
	r.Route("/api/admin/v1", adminH.Mount)
	r.Route("/api/kiosk/v1", kioskH.Mount)
	return r
}

func postJSON(t *testing.T, h http.Handler, path string, body any, cookie *http.Cookie) *http.Response {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		buf, _ := json.Marshal(body)
		rdr = bytes.NewReader(buf)
	}
	req := httptest.NewRequest(http.MethodPost, path, rdr)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w.Result()
}

func getJSON(t *testing.T, h http.Handler, path string, cookie *http.Cookie) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w.Result()
}

func TestE2E_OperatorFlow(t *testing.T) {
	legacy := fakeLegacyServer(t)
	defer legacy.Close()
	srv := buildKioskServer(t, legacy.URL)

	// 1. Login
	loginResp := postJSON(t, srv, "/api/admin/v1/auth/login",
		map[string]string{"username": "alice", "password": "wonderland"}, nil)
	if loginResp.StatusCode != 200 {
		t.Fatalf("login status = %d", loginResp.StatusCode)
	}
	cookie := loginResp.Cookies()[0]

	// 2. /me
	meResp := getJSON(t, srv, "/api/admin/v1/me", cookie)
	if meResp.StatusCode != 200 {
		t.Fatalf("me status = %d", meResp.StatusCode)
	}
	var me map[string]any
	_ = json.NewDecoder(meResp.Body).Decode(&me)
	if me["username"] != "alice" {
		t.Errorf("me.username = %v", me["username"])
	}

	// 3. List kiosks
	listResp := getJSON(t, srv, "/api/admin/v1/kiosks", cookie)
	if listResp.StatusCode != 200 {
		t.Fatalf("list status = %d", listResp.StatusCode)
	}
	var list struct {
		Kiosks []map[string]any `json:"kiosks"`
	}
	_ = json.NewDecoder(listResp.Body).Decode(&list)
	if len(list.Kiosks) != 1 || list.Kiosks[0]["uuid"] != "uuid-1" {
		t.Errorf("list = %+v", list)
	}

	// 4. Logout
	out := postJSON(t, srv, "/api/admin/v1/auth/logout", nil, cookie)
	if out.StatusCode != 204 {
		t.Errorf("logout = %d", out.StatusCode)
	}
}

func TestE2E_GuestFlow_FullCheckin(t *testing.T) {
	legacy := fakeLegacyServer(t)
	defer legacy.Close()
	srv := buildKioskServer(t, legacy.URL)

	// 1. /config — UUID validates via prestay_form on the legacy side
	cfg := getJSON(t, srv, "/api/kiosk/v1/uuid-1/config", nil)
	if cfg.StatusCode != 200 {
		t.Fatalf("config status = %d", cfg.StatusCode)
	}

	// 2. /lookup
	lookupResp := postJSON(t, srv, "/api/kiosk/v1/uuid-1/lookup",
		map[string]string{"search": "Pieper"}, nil)
	if lookupResp.StatusCode != 200 {
		t.Fatalf("lookup = %d", lookupResp.StatusCode)
	}
	var lookup struct {
		Result      string         `json:"result"`
		Token       string         `json:"token"`
		Reservation map[string]any `json:"reservation"`
	}
	_ = json.NewDecoder(lookupResp.Body).Decode(&lookup)
	if lookup.Result != "matched" {
		t.Fatalf("lookup result = %q, want matched", lookup.Result)
	}
	resID, _ := lookup.Reservation["code"].(string)
	if resID == "" {
		t.Fatalf("reservation.code empty")
	}

	// 3. /select
	sel := postJSON(t, srv, "/api/kiosk/v1/uuid-1/select",
		map[string]string{"reservationId": resID}, nil)
	if sel.StatusCode != 200 {
		t.Fatalf("select = %d", sel.StatusCode)
	}

	// 4. /form
	form := postJSON(t, srv, "/api/kiosk/v1/uuid-1/form",
		map[string]string{"reservationId": resID}, nil)
	if form.StatusCode != 200 {
		t.Fatalf("form = %d", form.StatusCode)
	}

	// 5. /save-guest
	saveG := postJSON(t, srv, "/api/kiosk/v1/uuid-1/save-guest", map[string]any{
		"reservationId": resID,
		"guest":         map[string]any{"fname": "Carmen", "lname": "Pieper"},
	}, nil)
	if saveG.StatusCode != 200 {
		t.Fatalf("save-guest = %d", saveG.StatusCode)
	}

	// 6. /save-firm — verify firm fields go flat upstream
	saveF := postJSON(t, srv, "/api/kiosk/v1/uuid-1/save-firm", map[string]any{
		"reservationId": resID,
		"firm":          map[string]any{"compname": "Acme", "vatid": "ATU123", "city": "Wien"},
	}, nil)
	if saveF.StatusCode != 200 {
		t.Fatalf("save-firm = %d", saveF.StatusCode)
	}

	// 7. /submit — no-op acknowledgement
	sub := postJSON(t, srv, "/api/kiosk/v1/uuid-1/submit", map[string]string{}, nil)
	if sub.StatusCode != 200 {
		t.Fatalf("submit = %d", sub.StatusCode)
	}

	// 8. /postal-lookup
	pl := postJSON(t, srv, "/api/kiosk/v1/uuid-1/postal-lookup",
		map[string]string{"postal": "1010", "country": "AT"}, nil)
	if pl.StatusCode != 200 {
		t.Fatalf("postal = %d", pl.StatusCode)
	}
	var plBody map[string]any
	_ = json.NewDecoder(pl.Body).Decode(&plBody)
	if plBody["city"] != "Wien" {
		t.Errorf("city = %v", plBody["city"])
	}
}

func TestE2E_UnknownKiosk_Returns410(t *testing.T) {
	legacy := fakeLegacyServer(t)
	defer legacy.Close()
	srv := buildKioskServer(t, legacy.URL)

	cfg := getJSON(t, srv, "/api/kiosk/v1/bogus-uuid/config", nil)
	if cfg.StatusCode != http.StatusGone {
		t.Errorf("config status = %d, want 410", cfg.StatusCode)
	}
}

func TestE2E_DeleteGuest(t *testing.T) {
	legacy := fakeLegacyServer(t)
	defer legacy.Close()
	srv := buildKioskServer(t, legacy.URL)

	resp := postJSON(t, srv, "/api/kiosk/v1/uuid-1/delete-guest",
		map[string]any{"reservationId": "ABC-1", "guestId": 42}, nil)
	if resp.StatusCode != 200 {
		t.Errorf("status = %d", resp.StatusCode)
	}
}
