// Package legacymaichttp implements the kiosk service's ports against
// the legacy MAIC PHP monolith at $LEGACY_BASE_URL (typically
// https://dev.maiccube.com). Routes live under /api/<snake_case> on the
// `newDev` branch — see routes/api.php:79-94 in MaicSystem/NewMAIC.
//
// The adapter is a thin HTTP client. It wraps Laravel's
// `BaseController::sendResponse` envelope ({...payload, status, status_message})
// so handlers see plain payloads. SPA-side wire shapes (camelCase, `firm:{}`
// envelope, `lookup`/`select` two-step) are translated here so the SPA
// bundle keeps working unchanged.
package legacymaichttp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/maic/checkin-kiosk-api/internal/domain"
	"github.com/maic/checkin-kiosk-api/internal/ports"
)

// Client is the single HTTP client shared by every legacy-side method.
// One instance per kiosk Pod; safe for concurrent use.
type Client struct {
	baseURL string
	http    *http.Client
}

// New builds a Client. baseURL is the legacy app's origin (no trailing
// slash). timeout caps every outbound call.
func New(baseURL string, timeout time.Duration) *Client {
	if timeout == 0 {
		timeout = 15 * time.Second
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:        20,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
				ForceAttemptHTTP2:   true,
			},
		},
	}
}

// Compile-time interface checks. Adding a new port? Add the assertion
// here so the compiler nags us if a method goes stale.
var (
	_ ports.OperatorAuth = (*Client)(nil)
	_ ports.KioskStore   = (*Client)(nil)
	_ ports.Reservations = (*Client)(nil)
	_ ports.Feratel      = (*Client)(nil)
)

// envelope mirrors Laravel's BaseController::sendResponse / sendError
// shape. We unwrap success and surface errors as domain errors.
type envelope struct {
	Status        int             `json:"status"`
	StatusMessage string          `json:"status_message"`
	// payload fields land at the top level alongside Status/StatusMessage,
	// so callers re-decode the same body into their own struct.
	raw json.RawMessage
}

// postJSON sends a JSON body to /api/<path> and returns the decoded
// payload (the full body, including status fields — caller picks the
// keys it cares about). out can be a *T or *map[string]any.
//
// If the upstream returns status==0, the call is treated as failure
// and ErrUpstream (wrapping the legacy status_message) is returned.
// HTTP non-2xx is mapped to ErrUpstream / ErrUnauthorized / ErrNotFound.
//
// extraHeaders lets the operator-flow paths replay the legacy session
// token when present — guest-flow paths pass nil.
func (c *Client) postJSON(ctx context.Context, path string, body any, out any, extraHeaders http.Header) error {
	buf, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/"+strings.TrimLeft(path, "/"), bytes.NewReader(buf))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	for k, vs := range extraHeaders {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}

	return c.do(req, out)
}

// postForm is the form-encoded variant used only by /api/login.
func (c *Client) postForm(ctx context.Context, path string, fields url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/"+strings.TrimLeft(path, "/"), strings.NewReader(fields.Encode()))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return c.do(req, out)
}

func (c *Client) do(req *http.Request, out any) error {
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", domain.ErrUpstream, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return fmt.Errorf("%w: read body: %v", domain.ErrUpstream, err)
	}

	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return fmt.Errorf("%w: %s", domain.ErrUnauthorized, string(body))
	case resp.StatusCode == http.StatusNotFound:
		return fmt.Errorf("%w: %s", domain.ErrNotFound, string(body))
	case resp.StatusCode >= 400:
		return fmt.Errorf("%w: HTTP %d: %s", domain.ErrUpstream, resp.StatusCode, string(body))
	}

	// Laravel's sendError uses status==0 even on HTTP 200; treat that
	// as a domain error too.
	var probe struct {
		Status        any    `json:"status"`
		StatusMessage string `json:"status_message"`
	}
	if err := json.Unmarshal(body, &probe); err == nil {
		// Status can be int or bool depending on the controller — be lenient.
		if isFalsy(probe.Status) && probe.StatusMessage != "" {
			return fmt.Errorf("%w: %s", domain.ErrUpstream, probe.StatusMessage)
		}
	}

	if out == nil {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("%w: decode response: %v", domain.ErrUpstream, err)
	}
	return nil
}

func isFalsy(v any) bool {
	switch x := v.(type) {
	case float64:
		return x == 0
	case int:
		return x == 0
	case bool:
		return !x
	case string:
		return x == "" || x == "0" || x == "false"
	case nil:
		return true
	}
	return false
}
