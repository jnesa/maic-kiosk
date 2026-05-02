package session_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/maic/checkin-kiosk-api/internal/domain"
	"github.com/maic/checkin-kiosk-api/internal/session"
)

const testSecret = "0123456789abcdef0123456789abcdef"

func TestIssueAndVerify_Roundtrip(t *testing.T) {
	sm := session.NewManager(testSecret)

	w := httptest.NewRecorder()
	op := domain.Operator{UserID: 7, Username: "alice", Email: "a@x", Name: "Alice", LegacySessionToken: "tok-1"}
	if err := sm.Issue(w, op); err != nil {
		t.Fatalf("issue: %v", err)
	}

	cookie := w.Result().Cookies()
	if len(cookie) != 1 || cookie[0].Name != session.CookieName {
		t.Fatalf("expected one cookie %q, got %+v", session.CookieName, cookie)
	}
	if !cookie[0].HttpOnly {
		t.Error("cookie should be HttpOnly")
	}

	got, err := sm.Verify(cookie[0].Value)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if got.UserID != 7 || got.LegacySessionToken != "tok-1" {
		t.Errorf("verify returned %+v", got)
	}
}

func TestVerify_TamperedSignature(t *testing.T) {
	sm := session.NewManager(testSecret)
	w := httptest.NewRecorder()
	op := domain.Operator{UserID: 1, LegacySessionToken: "t"}
	_ = sm.Issue(w, op)
	raw := w.Result().Cookies()[0].Value

	// Flip the last char of the signature; verification must fail.
	tampered := raw[:len(raw)-1] + "X"
	if _, err := sm.Verify(tampered); err == nil {
		t.Fatal("expected verification to fail after tamper")
	}
}

func TestVerify_DifferentSecret(t *testing.T) {
	smA := session.NewManager(testSecret)
	smB := session.NewManager("different-secret-1234567890abcd")

	w := httptest.NewRecorder()
	op := domain.Operator{UserID: 1, LegacySessionToken: "t"}
	_ = smA.Issue(w, op)
	raw := w.Result().Cookies()[0].Value

	if _, err := smB.Verify(raw); err == nil {
		t.Fatal("cookie signed by smA must not verify under smB")
	}
}

func TestMiddleware_NoCookie_Anonymous(t *testing.T) {
	sm := session.NewManager(testSecret)
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	w := httptest.NewRecorder()

	called := false
	sm.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if op := session.FromContext(r.Context()); op != nil {
			t.Errorf("expected anonymous, got operator %+v", op)
		}
	})).ServeHTTP(w, req)

	if !called {
		t.Fatal("downstream handler not called")
	}
}

func TestMiddleware_ValidCookie_PinsOperator(t *testing.T) {
	sm := session.NewManager(testSecret)
	rec := httptest.NewRecorder()
	op := domain.Operator{UserID: 9, LegacySessionToken: "t"}
	_ = sm.Issue(rec, op)
	cookie := rec.Result().Cookies()[0]

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()

	var pinned *domain.Operator
	sm.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pinned = session.FromContext(r.Context())
	})).ServeHTTP(w, req)

	if pinned == nil || pinned.UserID != 9 {
		t.Fatalf("expected pinned operator UserID=9, got %+v", pinned)
	}
}

func TestRequireOperator_RejectsAnonymous(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	w := httptest.NewRecorder()

	session.RequireOperator(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("downstream should not be called for anonymous request")
	})).ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestRequireOperator_AllowsAuthed(t *testing.T) {
	op := &domain.Operator{UserID: 1}
	req := httptest.NewRequest(http.MethodGet, "/x", nil).
		WithContext(contextWithOperator(op))
	w := httptest.NewRecorder()

	called := false
	session.RequireOperator(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})).ServeHTTP(w, req)

	if !called {
		t.Fatal("downstream should be called for authed request")
	}
}

// contextWithOperator simulates Middleware pinning by re-using the
// session.FromContext path indirectly. We do this by issuing+verifying
// to get a cookie, then running it through the middleware.
func contextWithOperator(op *domain.Operator) context.Context {
	sm := session.NewManager(testSecret)
	rec := httptest.NewRecorder()
	_ = sm.Issue(rec, *op)
	cookie := rec.Result().Cookies()[0]
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.AddCookie(cookie)

	var ctx context.Context
	sm.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx = r.Context()
	})).ServeHTTP(httptest.NewRecorder(), req)
	return ctx
}

func TestPanicsOnShortSecret(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on short secret")
		}
	}()
	session.NewManager("short")
}

// Quick integration: confirm Clear writes a delete cookie.
func TestClear_SetsDeleteCookie(t *testing.T) {
	sm := session.NewManager(testSecret)
	w := httptest.NewRecorder()
	sm.Clear(w)
	got := w.Result().Header.Get("Set-Cookie")
	if !strings.Contains(got, session.CookieName+"=;") {
		t.Errorf("expected delete cookie, got %q", got)
	}
	if !strings.Contains(got, "Max-Age=0") {
		t.Errorf("expected Max-Age=0, got %q", got)
	}
}
