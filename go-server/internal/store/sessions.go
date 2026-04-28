package store

import (
	"context"
	"database/sql"
	"errors"
)

// Session is one cookie-backed admin login. Token is the random hex value
// stored verbatim in the user's cookie.
type Session struct {
	Token       string
	UserID      int64
	CreatedAt   string
	ExpiresAt   string
	UserAgent   *string
}

// ErrSessionNotFound is returned by FindSession on miss / expiry / revoke.
var ErrSessionNotFound = errors.New("session not found")

// CreateSession persists a new session. Caller picks the token (so the
// cookie can be set on the response immediately after this returns).
func (s *Store) CreateSession(ctx context.Context, token string, userID int64, expiresAt, userAgent string) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO admin_sessions (token, admin_user_id, created_at, expires_at, user_agent)
VALUES (?, ?, ?, ?, ?)`, token, userID, nowISO(), expiresAt, nullable(userAgent))
	return err
}

// FindSession resolves a cookie token to its session row. Expired rows
// return ErrSessionNotFound — the caller treats them as anonymous.
func (s *Store) FindSession(ctx context.Context, token string) (*Session, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT token, admin_user_id, created_at, expires_at, user_agent
FROM admin_sessions
WHERE token = ? AND expires_at > strftime('%Y-%m-%dT%H:%M:%fZ', 'now')`, token)
	var sess Session
	var ua sql.NullString
	if err := row.Scan(&sess.Token, &sess.UserID, &sess.CreatedAt, &sess.ExpiresAt, &ua); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}
	if ua.Valid {
		v := ua.String
		sess.UserAgent = &v
	}
	return &sess, nil
}

// SlideSession refreshes an active session's expiry. Called on every
// authed request so an active operator stays logged in.
func (s *Store) SlideSession(ctx context.Context, token, newExpiresAt string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE admin_sessions SET expires_at = ? WHERE token = ?`, newExpiresAt, token)
	return err
}

// DeleteSession revokes a single session (logout).
func (s *Store) DeleteSession(ctx context.Context, token string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM admin_sessions WHERE token = ?`, token)
	return err
}

// DeleteSessionsForUser revokes every session for one user. Called when an
// admin disables a user or rotates their own password.
func (s *Store) DeleteSessionsForUser(ctx context.Context, userID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM admin_sessions WHERE admin_user_id = ?`, userID)
	return err
}

// PurgeExpiredSessions is called periodically (or on boot) to reclaim
// space. Cheap on SQLite at this scale.
func (s *Store) PurgeExpiredSessions(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
DELETE FROM admin_sessions
WHERE expires_at <= strftime('%Y-%m-%dT%H:%M:%fZ', 'now')`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// nullable converts an empty string to SQL NULL so we don't store noise
// in optional columns.
func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
