package admin_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/maic/checkin-kiosk-api/internal/adapters/fakes"
	"github.com/maic/checkin-kiosk-api/internal/admin"
	"github.com/maic/checkin-kiosk-api/internal/session"
)

const testSecret = "0123456789abcdef0123456789abcdef"

// newTestServer wires admin handlers on a chi router with the
// session middleware mounted. Returns the server and the auth/store
// fakes so tests can introspect.
func newTestServer(t *testing.T) (http.Handler, *fakes.OperatorAuth, *fakes.KioskStore, *session.Manager) {
	t.Helper()
	auth := fakes.NewOperatorAuth()
	ks := fakes.NewKioskStore()
	sm := session.NewManager(testSecret)
	h := admin.New(auth, ks, sm)

	r := chi.NewRouter()
	r.Use(sm.Middleware)
	r.Route("/api/admin/v1", h.Mount)
	return r, auth, ks, sm
}

func doJSON(t *testing.T, h http.Handler, method, path string, body any, cookie *http.Cookie) *http.Response {
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
	if cookie != nil {
		req.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w.Result()
}

func TestLogin_Success_SetsCookie(t *testing.T) {
	srv, _, _, _ := newTestServer(t)

	resp := doJSON(t, srv, http.MethodPost, "/api/admin/v1/auth/login",
		map[string]string{"username": "alice", "password": "wonderland"}, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("login status = %d", resp.StatusCode)
	}
	cookies := resp.Cookies()
	if len(cookies) == 0 || cookies[0].Name != session.CookieName {
		t.Fatalf("expected %q cookie, got %+v", session.CookieName, cookies)
	}
	if !cookies[0].HttpOnly {
		t.Error("session cookie must be HttpOnly")
	}
}

func TestLogin_Wrong_Returns401(t *testing.T) {
	srv, _, _, _ := newTestServer(t)
	resp := doJSON(t, srv, http.MethodPost, "/api/admin/v1/auth/login",
		map[string]string{"username": "alice", "password": "wrong"}, nil)
	if resp.StatusCode != 401 {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestLogin_MissingFields_Returns400(t *testing.T) {
	srv, _, _, _ := newTestServer(t)
	resp := doJSON(t, srv, http.MethodPost, "/api/admin/v1/auth/login",
		map[string]string{"username": ""}, nil)
	if resp.StatusCode != 400 {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestMe_RequiresAuth(t *testing.T) {
	srv, _, _, _ := newTestServer(t)
	resp := doJSON(t, srv, http.MethodGet, "/api/admin/v1/me", nil, nil)
	if resp.StatusCode != 401 {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestMe_AfterLogin_ReturnsOperator(t *testing.T) {
	srv, _, _, _ := newTestServer(t)
	loginResp := doJSON(t, srv, http.MethodPost, "/api/admin/v1/auth/login",
		map[string]string{"username": "alice", "password": "wonderland"}, nil)
	cookie := loginResp.Cookies()[0]

	resp := doJSON(t, srv, http.MethodGet, "/api/admin/v1/me", nil, cookie)
	if resp.StatusCode != 200 {
		t.Fatalf("/me status = %d", resp.StatusCode)
	}
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body["username"] != "alice" {
		t.Errorf("username = %v", body["username"])
	}
}

func TestListKiosks_RequiresAuth(t *testing.T) {
	srv, _, _, _ := newTestServer(t)
	resp := doJSON(t, srv, http.MethodGet, "/api/admin/v1/kiosks", nil, nil)
	if resp.StatusCode != 401 {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestListKiosks_ReturnsFakeData(t *testing.T) {
	srv, _, _, _ := newTestServer(t)
	loginResp := doJSON(t, srv, http.MethodPost, "/api/admin/v1/auth/login",
		map[string]string{"username": "alice", "password": "wonderland"}, nil)
	cookie := loginResp.Cookies()[0]

	resp := doJSON(t, srv, http.MethodGet, "/api/admin/v1/kiosks", nil, cookie)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var body struct {
		Kiosks []map[string]any `json:"kiosks"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if len(body.Kiosks) != 2 {
		t.Errorf("got %d kiosks, want 2", len(body.Kiosks))
	}
}

func TestLogout_ClearsCookie(t *testing.T) {
	srv, _, _, _ := newTestServer(t)
	loginResp := doJSON(t, srv, http.MethodPost, "/api/admin/v1/auth/login",
		map[string]string{"username": "alice", "password": "wonderland"}, nil)
	cookie := loginResp.Cookies()[0]

	resp := doJSON(t, srv, http.MethodPost, "/api/admin/v1/auth/logout", nil, cookie)
	if resp.StatusCode != 204 {
		t.Errorf("status = %d, want 204", resp.StatusCode)
	}
	clear := resp.Cookies()
	if len(clear) == 0 || clear[0].MaxAge != -1 {
		t.Errorf("expected delete cookie, got %+v", clear)
	}
}
