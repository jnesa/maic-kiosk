package legacymaichttp

import (
	"context"
	"net/url"

	"github.com/maic/checkin-kiosk-api/internal/domain"
)

// Login forwards form-encoded credentials to /api/login (legacy newDev
// `API\UsersController@userlogin`). The legacy app responds with a
// session token (Bearer JWT via Tymon JWTAuth, in current builds).
//
// We read the response leniently: any of `access_token`, `token`, or
// `auth.access_token` work, plus the user record under `user` /
// `userdata` / top-level. This keeps the adapter robust to small
// drift in the legacy response shape across deploys.
func (c *Client) Login(ctx context.Context, username, password, deviceID string) (*domain.LoginResult, error) {
	form := url.Values{}
	form.Set("username", username)
	form.Set("password", password)
	form.Set("device_id", deviceID)
	form.Set("action", "login")

	var resp map[string]any
	if err := c.postForm(ctx, "/login", form, &resp); err != nil {
		return nil, err
	}

	op := domain.Operator{
		LegacySessionToken: pickString(resp, "access_token", "token", "auth_token"),
		Username:           username,
	}

	// Try a few common shapes for the user record.
	if u, ok := resp["user"].(map[string]any); ok {
		op.UserID = pickInt64(u, "id", "user_id", "id_broker")
		op.Email = pickString(u, "email")
		op.Name = pickString(u, "name", "fullname")
	} else if u, ok := resp["userdata"].(map[string]any); ok {
		op.UserID = pickInt64(u, "id", "user_id", "id_broker")
		op.Email = pickString(u, "email")
		op.Name = pickString(u, "name", "fullname")
	} else {
		// Top-level fallbacks (some builds inline the user fields).
		op.UserID = pickInt64(resp, "user_id", "id", "id_broker")
		op.Email = pickString(resp, "email")
		op.Name = pickString(resp, "name", "fullname")
	}

	if op.LegacySessionToken == "" && op.UserID == 0 {
		return nil, domain.ErrUnauthorized
	}

	return &domain.LoginResult{Operator: op}, nil
}

func pickString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

func pickInt64(m map[string]any, keys ...string) int64 {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch x := v.(type) {
			case float64:
				return int64(x)
			case int64:
				return x
			case int:
				return int64(x)
			}
		}
	}
	return 0
}
