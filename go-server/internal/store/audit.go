package store

import (
	"context"
	"database/sql"
	"encoding/json"
)

// AuditEntry is one row from `audit_log`. Returned to the admin SPA on
// the activity page.
type AuditEntry struct {
	ID          int64
	UserID      *int64
	ActorEmail  *string
	Action      string
	EntityType  string
	EntityID    string
	Payload     map[string]any // decoded from the TEXT column; nil if empty
	CreatedAt   string
}

// LogAudit writes one row. Secrets must already be stripped from the
// payload by the handler — this layer does no scrubbing.
func (s *Store) LogAudit(ctx context.Context, userID *int64, actorEmail string, action, entityType, entityID string, payload map[string]any) error {
	var payloadJSON sql.NullString
	if len(payload) > 0 {
		b, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		payloadJSON = sql.NullString{String: string(b), Valid: true}
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO audit_log (admin_user_id, actor_email, action, entity_type, entity_id, payload, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?)`, userID, nullable(actorEmail), action, entityType, entityID, payloadJSON, nowISO())
	return err
}

// ListAudit returns the most recent entries, optionally paginated by id.
// `before` is the smallest id from the previous page (or 0 for newest).
func (s *Store) ListAudit(ctx context.Context, limit int, before int64) ([]AuditEntry, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	args := []any{}
	q := `
SELECT id, admin_user_id, actor_email, action, entity_type, entity_id, payload, created_at
FROM audit_log`
	if before > 0 {
		q += ` WHERE id < ?`
		args = append(args, before)
	}
	q += ` ORDER BY id DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditEntry
	for rows.Next() {
		var e AuditEntry
		var userID sql.NullInt64
		var actorEmail, payload sql.NullString
		if err := rows.Scan(&e.ID, &userID, &actorEmail, &e.Action, &e.EntityType, &e.EntityID, &payload, &e.CreatedAt); err != nil {
			return nil, err
		}
		if userID.Valid {
			v := userID.Int64
			e.UserID = &v
		}
		if actorEmail.Valid {
			v := actorEmail.String
			e.ActorEmail = &v
		}
		if payload.Valid && payload.String != "" {
			_ = json.Unmarshal([]byte(payload.String), &e.Payload)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
