// Package proxy forwards kiosk SPA requests to the property's legacy
// PMSApi /api/kiosk/* endpoints.
//
// The browser only ever sends requests to the kiosk backend (this
// service); the device key per kiosk lives here, never in the SPA
// bundle. We allowlist the legacy paths so the proxy can't be abused
// as an open relay.
package proxy

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// LegacyPath is one of the endpoints we expose, mapped to the legacy
// `/api/kiosk/<endpoint>` it forwards to.
type LegacyPath string

const (
	PathLookup    LegacyPath = "lookup"
	PathSelect    LegacyPath = "select"
	PathForm      LegacyPath = "form"
	PathSaveGuest LegacyPath = "save-guest"
	PathSaveFirm  LegacyPath = "save-firm"
	PathSubmit    LegacyPath = "submit"
)

// Allowed is the full forwarding allowlist. New legacy endpoints must
// be added here explicitly.
var Allowed = []LegacyPath{
	PathLookup, PathSelect, PathForm, PathSaveGuest, PathSaveFirm, PathSubmit,
}

// Target is the per-request shape we need from the caller. Decoupling
// from any concrete struct (`config.Property`, `store.Kiosk`) keeps
// the proxy useful no matter where the registry lives.
type Target struct {
	PmsApiURL string // base URL of the property's legacy PMSApi
	DeviceKey string // the secret to send as X-Device-Key
}

// Proxy holds the shared HTTP client + per-request timeout.
type Proxy struct {
	client  *http.Client
	timeout time.Duration
}

// New builds a Proxy with a finite timeout and a sensible connection
// pool.
func New(timeout time.Duration) *Proxy {
	return &Proxy{
		client: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:        20,
				MaxIdleConnsPerHost: 5,
				IdleConnTimeout:     90 * time.Second,
				ForceAttemptHTTP2:   true,
			},
		},
		timeout: timeout,
	}
}

// Forward sends the inbound request body to the target's
// `/api/kiosk/<path>` endpoint and copies the response back.
//
// Headers carried over: Content-Type, Accept-Language, X-Lookup-Method,
// X-Kiosk-Language, plus an X-Forwarded-For chain. Everything else is
// dropped so we don't leak cookies or auth headers to the legacy host.
func (p *Proxy) Forward(w http.ResponseWriter, r *http.Request, target Target, path LegacyPath) error {
	if !isAllowed(path) {
		return fmt.Errorf("path %q is not allowlisted", path)
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 512*1024))
	if err != nil {
		return fmt.Errorf("read inbound body: %w", err)
	}

	upstreamURL := strings.TrimRight(target.PmsApiURL, "/") + "/api/kiosk/" + string(path)
	upstream, err := http.NewRequestWithContext(r.Context(), r.Method, upstreamURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build upstream request: %w", err)
	}

	upstream.Header.Set("X-Device-Key", target.DeviceKey)
	upstream.Header.Set("Content-Type", inboundOr(r, "Content-Type", "application/json"))
	upstream.Header.Set("Accept", "application/json")

	if v := r.Header.Get("Accept-Language"); v != "" {
		upstream.Header.Set("Accept-Language", v)
	}
	if v := clientIP(r); v != "" {
		upstream.Header.Set("X-Forwarded-For", v)
	}
	if v := r.Header.Get("X-Lookup-Method"); v != "" {
		upstream.Header.Set("X-Lookup-Method", v)
	}
	if v := r.Header.Get("X-Kiosk-Language"); v != "" {
		upstream.Header.Set("X-Kiosk-Language", v)
	}

	resp, err := p.client.Do(upstream)
	if err != nil {
		var ne net.Error
		if errors.As(err, &ne) && ne.Timeout() {
			return fmt.Errorf("upstream timeout after %s: %w", p.timeout, err)
		}
		return fmt.Errorf("upstream request failed: %w", err)
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", respHeaderOr(resp, "Content-Type", "application/json"))
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
	return nil
}

func isAllowed(p LegacyPath) bool {
	for _, a := range Allowed {
		if a == p {
			return true
		}
	}
	return false
}

func inboundOr(r *http.Request, key, fallback string) string {
	if v := r.Header.Get(key); v != "" {
		return v
	}
	return fallback
}

func respHeaderOr(resp *http.Response, key, fallback string) string {
	if v := resp.Header.Get(key); v != "" {
		return v
	}
	return fallback
}

func clientIP(r *http.Request) string {
	if v := r.Header.Get("X-Forwarded-For"); v != "" {
		if i := strings.Index(v, ","); i >= 0 {
			return strings.TrimSpace(v[:i])
		}
		return strings.TrimSpace(v)
	}
	if v := r.Header.Get("X-Real-IP"); v != "" {
		return v
	}
	return r.RemoteAddr
}
