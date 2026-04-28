package store

import (
	"context"
	"database/sql"
	"errors"
)

// AdminUser is one row from `admin_users`. Not exported to admin SPA
// callers verbatim — handlers strip `PasswordHash` first.
type AdminUser struct {
	ID           int64
	Email        string
	Name         string
	PasswordHash string
	Status       string // "active" | "disabled"
	CreatedAt    string
	LastLoginAt  *string
}

// ErrUserNotFound is returned by lookups when no row matches.
var ErrUserNotFound = errors.New("admin user not found")

// CreateUser inserts a new admin user. The caller is responsible for
// bcrypt-hashing the password — this layer just stores opaque strings.
func (s *Store) CreateUser(ctx context.Context, email, name, passwordHash string) (*AdminUser, error) {
	now := nowISO()
	res, err := s.db.ExecContext(ctx, `
INSERT INTO admin_users (email, name, password_hash, status, created_at)
VALUES (?, ?, ?, 'active', ?)`, email, name, passwordHash, now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &AdminUser{
		ID: id, Email: email, Name: name, PasswordHash: passwordHash,
		Status: "active", CreatedAt: now,
	}, nil
}

// FindUserByEmail powers the login flow. Returns ErrUserNotFound on miss.
func (s *Store) FindUserByEmail(ctx context.Context, email string) (*AdminUser, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, email, name, password_hash, status, created_at, last_login_at
FROM admin_users WHERE email = ? COLLATE NOCASE`, email)
	return scanUser(row)
}

// FindUserByID powers /me and session loads.
func (s *Store) FindUserByID(ctx context.Context, id int64) (*AdminUser, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, email, name, password_hash, status, created_at, last_login_at
FROM admin_users WHERE id = ?`, id)
	return scanUser(row)
}

// SetPasswordHash is called by the password-reset CLI. Audit-logged at the
// caller, not here.
func (s *Store) SetPasswordHash(ctx context.Context, id int64, hash string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE admin_users SET password_hash = ? WHERE id = ?`, hash, id)
	return err
}

// SetUserStatus toggles between 'active' and 'disabled'. Disabled users
// can't log in (login handler checks) and existing sessions are
// invalidated by the caller.
func (s *Store) SetUserStatus(ctx context.Context, id int64, status string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE admin_users SET status = ? WHERE id = ?`, status, id)
	return err
}

// TouchLogin records the last successful login timestamp. Best-effort —
// failure here doesn't block the login itself.
func (s *Store) TouchLogin(ctx context.Context, id int64) {
	_, _ = s.db.ExecContext(ctx, `UPDATE admin_users SET last_login_at = ? WHERE id = ?`, nowISO(), id)
}

// ListUsers returns all rows, newest first. Used by the admin-cli list
// command and (eventually) a small users-management page.
func (s *Store) ListUsers(ctx context.Context) ([]AdminUser, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, email, name, password_hash, status, created_at, last_login_at
FROM admin_users ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AdminUser
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *u)
	}
	return out, rows.Err()
}

// scanUser is a tiny helper so QueryRow and Query share the same column
// list and conversion logic.
func scanUser(row interface{ Scan(...any) error }) (*AdminUser, error) {
	var u AdminUser
	var lastLogin sql.NullString
	if err := row.Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.Status, &u.CreatedAt, &lastLogin); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	if lastLogin.Valid {
		v := lastLogin.String
		u.LastLoginAt = &v
	}
	return &u, nil
}
