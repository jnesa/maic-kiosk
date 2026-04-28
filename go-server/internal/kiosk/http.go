// Package kiosk owns the public-facing /api/kiosk/v1/* HTTP routes.
//
// Lookups are by opaque kiosk UUID (the path the SPA puts in the URL).
// The handler resolves the kiosk via the SQLite store, then either
// returns the sanitised public config (`/config`) or proxies the
// request to the kiosk's hotel.pmsapi_url with the per-kiosk device
// key attached (`/lookup`, `/select`, etc.).
package kiosk

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/maic/checkin-kiosk-api/internal/proxy"
	"github.com/maic/checkin-kiosk-api/internal/store"
)

// Handler bundles the dependencies the kiosk routes need.
type Handler struct {
	Store *store.Store
	Proxy *proxy.Proxy
}

// New wires a Handler.
func New(s *store.Store, p *proxy.Proxy) *Handler {
	return &Handler{Store: s, Proxy: p}
}

// Mount attaches every public kiosk route. Auth is per-request and
// implicit: knowing the UUID is the entire authorisation surface for
// `/config`, and the legacy backend re-authorises every mutating call
// via `KIOSK_DEVICE_KEY`.
func (h *Handler) Mount(r chi.Router) {
	r.Get("/health", h.health)
	r.Get("/ready", h.ready)

	r.Route("/{id}", func(r chi.Router) {
		r.Use(h.kioskResolver)

		r.Get("/config", h.config)
		r.Post("/lookup", h.forward(proxy.PathLookup))
		r.Post("/select", h.forward(proxy.PathSelect))
		r.Post("/form", h.forward(proxy.PathForm))
		r.Post("/save-guest", h.forward(proxy.PathSaveGuest))
		r.Post("/save-firm", h.forward(proxy.PathSaveFirm))
		r.Post("/submit", h.forward(proxy.PathSubmit))
	})
}

func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "healthy", "service": "checkin-kiosk-api"})
}

func (h *Handler) ready(w http.ResponseWriter, r *http.Request) {
	if err := h.Store.DB().PingContext(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "not ready", "reason": "db unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ready"})
}

// config returns the sanitised public view of a kiosk. Disabled kiosks
// return 410 so the SPA can render a "deactivated" screen instead of
// looking like a generic network error.
func (h *Handler) config(w http.ResponseWriter, r *http.Request) {
	k := kioskFromCtx(r.Context())
	if k.Status == "disabled" {
		writeJSON(w, http.StatusGone, map[string]any{
			"error": map[string]string{"code": "kiosk_disabled", "message": "This kiosk is currently disabled."},
		})
		return
	}
	writeJSON(w, http.StatusOK, publicKioskView(k))
}

// forward returns an http.HandlerFunc that proxies the inbound request
// to the kiosk's hotel.pmsapi_url + the legacy /api/kiosk/<path>.
func (h *Handler) forward(path proxy.LegacyPath) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		k := kioskFromCtx(r.Context())
		if k.Status == "disabled" {
			writeJSON(w, http.StatusGone, map[string]any{
				"error": map[string]string{"code": "kiosk_disabled"},
			})
			return
		}
		err := h.Proxy.Forward(w, r, proxy.Target{
			PmsApiURL: k.HotelPmsApiURL,
			DeviceKey: k.DeviceKey,
		}, path)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]any{
				"error": map[string]string{"code": "upstream_failed", "message": err.Error()},
			})
		}
	}
}

// publicKioskView is what the SPA boots from. Strips device_key,
// hotel id, hotel.pmsapi_url, etc.
func publicKioskView(k *store.Kiosk) map[string]any {
	return map[string]any{
		"id":            k.ID,
		"name":          k.HotelName,        // hotel-level name; eg "smart moov"
		"displayName":   k.DisplayName,      // kiosk-level label; eg "lobby tablet"
		"theme":         k.Theme,
		"languages":     k.Languages,
		"heroImage":     k.HeroImage,
		"logo":          k.Logo,
		"supportPhone":  k.SupportPhone,
		"supportEmail":  k.SupportEmail,
	}
}

// -----------------------------------------------------------------------
// Resolver middleware
// -----------------------------------------------------------------------

type ctxKey int

const ctxKiosk ctxKey = iota + 1

// kioskResolver looks up the kiosk by its URL-path UUID and pins it
// into the request context. Unknown ids return 404; everything else
// passes through.
func (h *Handler) kioskResolver(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		k, err := h.Store.FindKioskWithHotel(r.Context(), id)
		if err != nil {
			if errors.Is(err, store.ErrKioskNotFound) {
				writeJSON(w, http.StatusNotFound, map[string]any{
					"error": map[string]string{"code": "unknown_kiosk", "message": "no kiosk for that id"},
				})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error": map[string]string{"code": "internal", "message": err.Error()},
			})
			return
		}
		ctx := context.WithValue(r.Context(), ctxKiosk, k)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func kioskFromCtx(ctx context.Context) *store.Kiosk {
	v, _ := ctx.Value(ctxKiosk).(*store.Kiosk)
	return v
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
