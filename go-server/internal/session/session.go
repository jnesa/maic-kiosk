// Package session manages the operator-side cookie that carries the
// legacy MAIC session token across requests. The kiosk Go service
// owns no session store of its own — it stores the legacy token
// inside an HMAC-signed cookie and replays it on subsequent calls.
//
// Cookie format: base64url(JSON{user_id, username, token, exp}) + "."
// + base64url(HMAC-SHA256). Stateless, tamper-evident, ~250 bytes.
package session

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/maic/checkin-kiosk-api/internal/domain"
)

// CookieName is the name of the operator-session cookie set by /auth/login.
const CookieName = "kiosk_admin_session"

// SessionTTL is the absolute lifetime of a cookie-encoded session.
// 12 hours, matching the legacy app's typical JWT lifetime.
const SessionTTL = 12 * time.Hour

// payload is the JSON-serialised cookie body (before signing).
type payload struct {
	UserID   int64  `json:"u"`
	Username string `json:"n"`
	Email    string `json:"e,omitempty"`
	FullName string `json:"f,omitempty"`
	Token    string `json:"t"`
	Exp      int64  `json:"x"`
}

// Manager signs/verifies cookies. One per process; safe for concurrent use.
type Manager struct {
	secret []byte
}

// NewManager initialises the cookie signer with a server-side secret.
// Use a 32+ byte random value in prod; rotation invalidates all
// existing sessions.
func NewManager(secret string) *Manager {
	if len(secret) < 16 {
		// Defensive — refuse to run with a too-short secret. Caller
		// generates a random 32-byte value if KIOSK_SESSION_SECRET
		// is unset.
		panic("session.NewManager: secret must be at least 16 bytes")
	}
	return &Manager{secret: []byte(secret)}
}

// Issue creates a signed cookie for the given operator. Sets it on w.
func (m *Manager) Issue(w http.ResponseWriter, op domain.Operator) error {
	p := payload{
		UserID:   op.UserID,
		Username: op.Username,
		Email:    op.Email,
		FullName: op.Name,
		Token:    op.LegacySessionToken,
		Exp:      time.Now().Add(SessionTTL).Unix(),
	}
	enc, err := m.encode(p)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    enc,
		Path:     "/",
		Expires:  time.Unix(p.Exp, 0),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

// Clear deletes the operator session cookie (logout).
func (m *Manager) Clear(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

// Verify decodes + verifies a cookie value. Returns the operator if
// the signature is valid and the cookie hasn't expired.
func (m *Manager) Verify(raw string) (*domain.Operator, error) {
	p, err := m.decode(raw)
	if err != nil {
		return nil, err
	}
	if time.Now().Unix() > p.Exp {
		return nil, errors.New("session expired")
	}
	return &domain.Operator{
		UserID:             p.UserID,
		Username:           p.Username,
		Email:              p.Email,
		Name:               p.FullName,
		LegacySessionToken: p.Token,
	}, nil
}

// Middleware loads the operator from the cookie (if any) and pins
// them into the request context. Routes that require auth use
// RequireOperator below.
func (m *Manager) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(CookieName)
		if err == nil && c.Value != "" {
			if op, err := m.Verify(c.Value); err == nil {
				r = r.WithContext(context.WithValue(r.Context(), ctxKeyOperator, op))
			} else {
				// Invalid/expired — clear so the SPA stops resending.
				m.Clear(w)
			}
		}
		next.ServeHTTP(w, r)
	})
}

// RequireOperator is a route-level guard. Pair with Middleware.
func RequireOperator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if FromContext(r.Context()) == nil {
			http.Error(w, `{"error":{"code":"unauthenticated"}}`, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type ctxKey int

const ctxKeyOperator ctxKey = 1

// FromContext returns the operator pinned by Middleware, or nil if
// the request is anonymous.
func FromContext(ctx context.Context) *domain.Operator {
	v, _ := ctx.Value(ctxKeyOperator).(*domain.Operator)
	return v
}

// --- encoding ---

func (m *Manager) encode(p payload) (string, error) {
	body, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	bodyB64 := base64.RawURLEncoding.EncodeToString(body)
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(bodyB64))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return bodyB64 + "." + sig, nil
}

func (m *Manager) decode(raw string) (*payload, error) {
	dot := -1
	for i := len(raw) - 1; i >= 0; i-- {
		if raw[i] == '.' {
			dot = i
			break
		}
	}
	if dot < 1 || dot == len(raw)-1 {
		return nil, errors.New("malformed cookie")
	}
	bodyB64, sigB64 := raw[:dot], raw[dot+1:]

	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(bodyB64))
	expectedSig := mac.Sum(nil)
	gotSig, err := base64.RawURLEncoding.DecodeString(sigB64)
	if err != nil || !hmac.Equal(gotSig, expectedSig) {
		return nil, errors.New("signature mismatch")
	}

	body, err := base64.RawURLEncoding.DecodeString(bodyB64)
	if err != nil {
		return nil, err
	}
	var p payload
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, err
	}
	return &p, nil
}
