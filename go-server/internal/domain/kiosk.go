// Package domain holds the pure business types that flow between the
// kiosk service's handlers and its adapter layer. No HTTP concerns, no
// transport tags, no DB tags — these structs are the contract every
// adapter must agree on.
package domain

// Kiosk is one self-check-in URL pinned to a single property (a legacy
// g_group). The legacy app's `App\Kiosk` Eloquent model is the source
// of truth — the kiosk Go service does not own a kiosks table.
type Kiosk struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	UUID      string `json:"uuid"`
	GroupID   int64  `json:"group_id"`
	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}
