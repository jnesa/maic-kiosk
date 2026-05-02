package domain

// Operator is an authenticated MAIC operator. The kiosk service does
// not own the user record — `LegacySessionToken` is opaque, captured
// from the legacy `/api/login` response and replayed on subsequent
// upstream calls.
type Operator struct {
	UserID             int64  `json:"user_id"`
	Username           string `json:"username,omitempty"`
	Email              string `json:"email,omitempty"`
	Name               string `json:"name,omitempty"`
	LegacySessionToken string `json:"-"` // never serialised to client
}

// LoginResult is what `OperatorAuth.Login` returns. The kiosk service
// stuffs the LegacySessionToken into a cookie and returns the public
// fields to the SPA.
type LoginResult struct {
	Operator Operator
}
