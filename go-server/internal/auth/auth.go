// Package auth handles operator authentication for the admin panel.
//
// Three concerns live here:
//   - Hashing + verifying passwords with bcrypt.
//   - Generating opaque session tokens.
//   - The HTTP middleware that loads a session from the cookie and
//     pins the user into the request context.
package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/maic/checkin-kiosk-api/internal/store"
)

// CookieName is the session cookie name used by the admin panel.
const CookieName = "kiosk_admin_session"

// SessionTTL is the absolute lifetime of a freshly minted session.
// Sliding refresh extends it; explicit logout or password rotation
// kills it earlier.
const SessionTTL = 12 * time.Hour

// Bcrypt cost. 12 is a fair middle ground in 2026 — ~250ms on modern
// hardware, fast enough for login UX, slow enough to make a leaked
// hash painful to brute-force.
const bcryptCost = 12

// HashPassword wraps bcrypt with our chosen cost so callers don't have
// to import the package directly.
func HashPassword(plain string) (string, error) {
	if plain == "" {
		return "", errors.New("password cannot be empty")
	}
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// VerifyPassword constant-time-compares a plaintext attempt against a
// stored hash. Returns nil on success.
func VerifyPassword(hash, attempt string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(attempt))
}

// NewSessionToken returns 32 random bytes hex-encoded. 256 bits of
// entropy — more than enough for a session id.
func NewSessionToken() (string, error) {
	var buf [32]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}

// SessionExpiry returns the ISO-8601 timestamp at which a session
// minted now should expire.
func SessionExpiry(now time.Time) string {
	return now.UTC().Add(SessionTTL).Format("2006-01-02T15:04:05.000Z")
}

// ctxKey is intentionally unexported so other packages can only access
// the user via the helpers below.
type ctxKey int

const (
	ctxUser ctxKey = iota + 1
	ctxSession
)

// Middleware loads a session from the request cookie and pins the user
// into the request context. Requests without a valid session pass
// through with no user — the route handler decides whether that's OK.
//
// Sliding refresh: if a session is more than half-used we extend it.
// Cheap on SQLite at admin volumes.
func Middleware(s *store.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, err := r.Cookie(CookieName)
			if err != nil || c.Value == "" {
				next.ServeHTTP(w, r)
				return
			}
			sess, err := s.FindSession(r.Context(), c.Value)
			if err != nil {
				// Expired or revoked — clear the cookie so the SPA
				// stops sending it.
				clearCookie(w)
				next.ServeHTTP(w, r)
				return
			}
			user, err := s.FindUserByID(r.Context(), sess.UserID)
			if err != nil || user.Status != "active" {
				clearCookie(w)
				next.ServeHTTP(w, r)
				return
			}

			// Sliding refresh — only when needed to keep the write
			// rate sane.
			if shouldSlide(sess.ExpiresAt) {
				newExp := SessionExpiry(time.Now())
				_ = s.SlideSession(r.Context(), sess.Token, newExp)
				setCookie(w, sess.Token, newExp)
			}

			ctx := context.WithValue(r.Context(), ctxUser, user)
			ctx = context.WithValue(ctx, ctxSession, sess)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireUser is a route-level guard. Use after Middleware. Responds
// 401 if no session was attached.
func RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if UserFromCtx(r.Context()) == nil {
			http.Error(w, `{"error":{"code":"unauthenticated"}}`, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// UserFromCtx returns the authed user pinned by Middleware, or nil if
// the request is anonymous.
func UserFromCtx(ctx context.Context) *store.AdminUser {
	v, _ := ctx.Value(ctxUser).(*store.AdminUser)
	return v
}

// SessionFromCtx returns the session row, or nil.
func SessionFromCtx(ctx context.Context) *store.Session {
	v, _ := ctx.Value(ctxSession).(*store.Session)
	return v
}

// SetSessionCookie writes the session cookie on a response. Used by the
// login handler immediately after CreateSession.
func SetSessionCookie(w http.ResponseWriter, token, expiresAt string) {
	setCookie(w, token, expiresAt)
}

// ClearSessionCookie removes the session cookie. Used by /logout.
func ClearSessionCookie(w http.ResponseWriter) {
	clearCookie(w)
}

func setCookie(w http.ResponseWriter, token, expiresAtISO string) {
	exp, _ := time.Parse("2006-01-02T15:04:05.000Z", expiresAtISO)
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		Expires:  exp,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearCookie(w http.ResponseWriter) {
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

// shouldSlide says yes when the session's remaining lifetime is less
// than half of the original TTL. Cap on write rate even for chatty
// operators — a session is only refreshed every ~6 hours.
func shouldSlide(expiresAtISO string) bool {
	exp, err := time.Parse("2006-01-02T15:04:05.000Z", expiresAtISO)
	if err != nil {
		return false
	}
	remaining := time.Until(exp)
	return remaining < SessionTTL/2
}
