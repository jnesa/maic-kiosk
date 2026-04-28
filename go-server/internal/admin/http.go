// Package admin holds the /api/admin/v1/* HTTP handlers.
package admin

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/maic/checkin-kiosk-api/internal/auth"
	"github.com/maic/checkin-kiosk-api/internal/store"
)

// Handler bundles the dependencies the admin routes need. Built once at
// boot and registered onto the chi router.
type Handler struct {
	Store *store.Store
}

// New wires a Handler with its dependencies.
func New(s *store.Store) *Handler { return &Handler{Store: s} }

// Mount attaches every admin route to the supplied router. The auth
// middleware is applied here — public-facing routes (`login`) are
// outside the RequireUser block.
func (h *Handler) Mount(r chi.Router) {
	r.Post("/auth/login", h.login)

	r.Group(func(r chi.Router) {
		r.Use(auth.RequireUser)

		r.Post("/auth/logout", h.logout)
		r.Get("/me", h.me)

		r.Get("/hotels", h.listHotels)
		r.Post("/hotels", h.createHotel)
		r.Get("/hotels/{id}", h.getHotel)
		r.Patch("/hotels/{id}", h.updateHotel)
		r.Delete("/hotels/{id}", h.deleteHotel)

		r.Post("/hotels/{id}/kiosks", h.createKiosk)

		r.Get("/kiosks/{id}", h.getKiosk)
		r.Patch("/kiosks/{id}", h.updateKiosk)
		r.Post("/kiosks/{id}/rotate-key", h.rotateKey)
		r.Post("/kiosks/{id}/disable", h.disableKiosk)
		r.Post("/kiosks/{id}/enable", h.enableKiosk)
		r.Delete("/kiosks/{id}", h.deleteKiosk)

		r.Get("/audit-log", h.listAudit)
	})
}

// -----------------------------------------------------------------------
// Auth
// -----------------------------------------------------------------------

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request", "invalid JSON")
		return
	}
	if req.Email == "" || req.Password == "" {
		writeErr(w, http.StatusBadRequest, "bad_request", "email and password required")
		return
	}

	user, err := h.Store.FindUserByEmail(r.Context(), req.Email)
	// We don't disclose whether the email exists — same response either way.
	if err != nil || user.Status != "active" || auth.VerifyPassword(user.PasswordHash, req.Password) != nil {
		writeErr(w, http.StatusUnauthorized, "invalid_credentials", "wrong email or password")
		return
	}

	token, err := auth.NewSessionToken()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	exp := auth.SessionExpiry(time.Now())
	if err := h.Store.CreateSession(r.Context(), token, user.ID, exp, r.UserAgent()); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	auth.SetSessionCookie(w, token, exp)

	h.audit(r.Context(), user, "login", "admin_user", strconv.FormatInt(user.ID, 10), nil)
	h.Store.TouchLogin(r.Context(), user.ID)

	writeJSON(w, http.StatusOK, map[string]any{"user": publicUser(user)})
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	sess := auth.SessionFromCtx(r.Context())
	user := auth.UserFromCtx(r.Context())
	if sess != nil {
		_ = h.Store.DeleteSession(r.Context(), sess.Token)
	}
	auth.ClearSessionCookie(w)
	h.audit(r.Context(), user, "logout", "admin_user", strconv.FormatInt(user.ID, 10), nil)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromCtx(r.Context())
	writeJSON(w, http.StatusOK, publicUser(user))
}

// -----------------------------------------------------------------------
// Hotels
// -----------------------------------------------------------------------

type hotelReq struct {
	Name      string `json:"name"`
	PmsApiURL string `json:"pmsapi_url"`
	Notes     string `json:"notes"`
}

func (h *Handler) listHotels(w http.ResponseWriter, r *http.Request) {
	hotels, err := h.Store.ListHotels(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"hotels": publicHotels(hotels)})
}

func (h *Handler) createHotel(w http.ResponseWriter, r *http.Request) {
	var req hotelReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request", "invalid JSON")
		return
	}
	if err := validateHotelInput(req.Name, req.PmsApiURL); err != nil {
		writeErr(w, http.StatusBadRequest, "validation", err.Error())
		return
	}
	hotel, err := h.Store.CreateHotel(r.Context(), req.Name, normaliseURL(req.PmsApiURL), req.Notes)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	user := auth.UserFromCtx(r.Context())
	h.audit(r.Context(), user, "create_hotel", "hotel", strconv.FormatInt(hotel.ID, 10), map[string]any{
		"name":       hotel.Name,
		"pmsapi_url": hotel.PmsApiURL,
	})
	writeJSON(w, http.StatusCreated, publicHotel(hotel, 0))
}

func (h *Handler) getHotel(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt(r, "id")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request", "id must be integer")
		return
	}
	hotel, err := h.Store.FindHotel(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "not_found", "hotel not found")
		return
	}
	kiosks, err := h.Store.ListKiosksByHotel(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	resp := publicHotel(hotel, len(kiosks))
	resp["kiosks"] = adminKioskList(kiosks)
	writeJSON(w, http.StatusOK, resp)
}

type hotelPatchReq struct {
	Name      *string `json:"name,omitempty"`
	PmsApiURL *string `json:"pmsapi_url,omitempty"`
	Notes     *string `json:"notes,omitempty"`
}

func (h *Handler) updateHotel(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt(r, "id")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request", "id must be integer")
		return
	}
	var req hotelPatchReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request", "invalid JSON")
		return
	}
	if req.PmsApiURL != nil {
		v := normaliseURL(*req.PmsApiURL)
		if !validURL(v) {
			writeErr(w, http.StatusBadRequest, "validation", "pmsapi_url must be http(s) URL")
			return
		}
		req.PmsApiURL = &v
	}
	hotel, err := h.Store.UpdateHotel(r.Context(), id, store.HotelPatch{
		Name: req.Name, PmsApiURL: req.PmsApiURL, Notes: req.Notes,
	})
	if err != nil {
		if errors.Is(err, store.ErrHotelNotFound) {
			writeErr(w, http.StatusNotFound, "not_found", "hotel not found")
			return
		}
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	user := auth.UserFromCtx(r.Context())
	h.audit(r.Context(), user, "update_hotel", "hotel", strconv.FormatInt(hotel.ID, 10), nil)
	writeJSON(w, http.StatusOK, publicHotel(hotel, 0))
}

func (h *Handler) deleteHotel(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt(r, "id")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request", "id must be integer")
		return
	}
	if _, err := h.Store.FindHotel(r.Context(), id); err != nil {
		writeErr(w, http.StatusNotFound, "not_found", "hotel not found")
		return
	}
	if err := h.Store.DeleteHotel(r.Context(), id); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	user := auth.UserFromCtx(r.Context())
	h.audit(r.Context(), user, "delete_hotel", "hotel", strconv.FormatInt(id, 10), nil)
	w.WriteHeader(http.StatusNoContent)
}

// -----------------------------------------------------------------------
// Kiosks
// -----------------------------------------------------------------------

type kioskReq struct {
	DisplayName      string   `json:"display_name"`
	LegacyGroupID    *int64   `json:"legacy_group_id,omitempty"`
	LegacyGroupLabel string   `json:"legacy_group_label"`
	Theme            string   `json:"theme"`
	Languages        []string `json:"languages"`
	HeroImage        string   `json:"hero_image"`
	Logo             string   `json:"logo"`
	SupportPhone     string   `json:"support_phone"`
	SupportEmail     string   `json:"support_email"`
}

func (h *Handler) createKiosk(w http.ResponseWriter, r *http.Request) {
	hotelID, err := pathInt(r, "id")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request", "id must be integer")
		return
	}
	if _, err := h.Store.FindHotel(r.Context(), hotelID); err != nil {
		writeErr(w, http.StatusNotFound, "not_found", "hotel not found")
		return
	}
	var req kioskReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request", "invalid JSON")
		return
	}
	if err := validateKioskInput(req.DisplayName, req.Theme, req.Languages); err != nil {
		writeErr(w, http.StatusBadRequest, "validation", err.Error())
		return
	}
	id, err := newKioskID()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	deviceKey, err := newDeviceKey()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	kiosk, err := h.Store.CreateKiosk(r.Context(), id, hotelID, deviceKey, store.KioskInput{
		DisplayName:      req.DisplayName,
		LegacyGroupID:    req.LegacyGroupID,
		LegacyGroupLabel: req.LegacyGroupLabel,
		Theme:            req.Theme,
		Languages:        req.Languages,
		HeroImage:        req.HeroImage,
		Logo:             req.Logo,
		SupportPhone:     req.SupportPhone,
		SupportEmail:     req.SupportEmail,
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	user := auth.UserFromCtx(r.Context())
	h.audit(r.Context(), user, "create_kiosk", "kiosk", kiosk.ID, map[string]any{
		"display_name":   kiosk.DisplayName,
		"hotel_id":       hotelID,
		"theme":          kiosk.Theme,
		"legacy_group":   kiosk.LegacyGroupID,
	})
	writeJSON(w, http.StatusCreated, adminKiosk(kiosk))
}

func (h *Handler) getKiosk(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	kiosk, err := h.Store.FindKiosk(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "not_found", "kiosk not found")
		return
	}
	writeJSON(w, http.StatusOK, adminKiosk(kiosk))
}

type kioskPatchReq struct {
	DisplayName      *string  `json:"display_name,omitempty"`
	LegacyGroupID    *int64   `json:"legacy_group_id,omitempty"`
	LegacyGroupLabel *string  `json:"legacy_group_label,omitempty"`
	Theme            *string  `json:"theme,omitempty"`
	Languages        []string `json:"languages,omitempty"`
	HeroImage        *string  `json:"hero_image,omitempty"`
	Logo             *string  `json:"logo,omitempty"`
	SupportPhone     *string  `json:"support_phone,omitempty"`
	SupportEmail     *string  `json:"support_email,omitempty"`
}

func (h *Handler) updateKiosk(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req kioskPatchReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request", "invalid JSON")
		return
	}
	kiosk, err := h.Store.UpdateKiosk(r.Context(), id, store.KioskPatch{
		DisplayName:      req.DisplayName,
		LegacyGroupID:    req.LegacyGroupID,
		LegacyGroupLabel: req.LegacyGroupLabel,
		Theme:            req.Theme,
		Languages:        req.Languages,
		HeroImage:        req.HeroImage,
		Logo:             req.Logo,
		SupportPhone:     req.SupportPhone,
		SupportEmail:     req.SupportEmail,
	})
	if err != nil {
		if errors.Is(err, store.ErrKioskNotFound) {
			writeErr(w, http.StatusNotFound, "not_found", "kiosk not found")
			return
		}
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	user := auth.UserFromCtx(r.Context())
	h.audit(r.Context(), user, "update_kiosk", "kiosk", kiosk.ID, nil)
	writeJSON(w, http.StatusOK, adminKiosk(kiosk))
}

func (h *Handler) rotateKey(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := h.Store.FindKiosk(r.Context(), id); err != nil {
		writeErr(w, http.StatusNotFound, "not_found", "kiosk not found")
		return
	}
	newKey, err := newDeviceKey()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if err := h.Store.RotateKioskKey(r.Context(), id, newKey); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	user := auth.UserFromCtx(r.Context())
	h.audit(r.Context(), user, "rotate_key", "kiosk", id, nil)
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "device_key": newKey})
}

func (h *Handler) disableKiosk(w http.ResponseWriter, r *http.Request) {
	h.setStatus(w, r, "disabled", "disable_kiosk")
}

func (h *Handler) enableKiosk(w http.ResponseWriter, r *http.Request) {
	h.setStatus(w, r, "active", "enable_kiosk")
}

func (h *Handler) setStatus(w http.ResponseWriter, r *http.Request, status, action string) {
	id := chi.URLParam(r, "id")
	kiosk, err := h.Store.FindKiosk(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "not_found", "kiosk not found")
		return
	}
	if err := h.Store.SetKioskStatus(r.Context(), id, status); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	user := auth.UserFromCtx(r.Context())
	h.audit(r.Context(), user, action, "kiosk", id, nil)
	kiosk.Status = status
	writeJSON(w, http.StatusOK, adminKiosk(kiosk))
}

func (h *Handler) deleteKiosk(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := h.Store.FindKiosk(r.Context(), id); err != nil {
		writeErr(w, http.StatusNotFound, "not_found", "kiosk not found")
		return
	}
	if err := h.Store.DeleteKiosk(r.Context(), id); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	user := auth.UserFromCtx(r.Context())
	h.audit(r.Context(), user, "delete_kiosk", "kiosk", id, nil)
	w.WriteHeader(http.StatusNoContent)
}

// -----------------------------------------------------------------------
// Audit log
// -----------------------------------------------------------------------

func (h *Handler) listAudit(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	before, _ := strconv.ParseInt(r.URL.Query().Get("before"), 10, 64)
	entries, err := h.Store.ListAudit(r.Context(), limit, before)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries})
}

// audit is the convenience wrapper handlers use. Strips secrets is the
// caller's job — handlers above either pass nil or a deliberately
// scrubbed payload.
func (h *Handler) audit(ctx context.Context, user *store.AdminUser, action, entityType, entityID string, payload map[string]any) {
	var userID *int64
	var email string
	if user != nil {
		userID = &user.ID
		email = user.Email
	}
	_ = h.Store.LogAudit(ctx, userID, email, action, entityType, entityID, payload)
}

// -----------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": msg}})
}

func pathInt(r *http.Request, name string) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, name), 10, 64)
}

func newKioskID() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return "k_" + hex.EncodeToString(buf[:]), nil
}

func newDeviceKey() (string, error) {
	var buf [32]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}

// publicUser strips the password hash before serialising.
func publicUser(u *store.AdminUser) map[string]any {
	if u == nil {
		return nil
	}
	return map[string]any{
		"id":            u.ID,
		"email":         u.Email,
		"name":          u.Name,
		"status":        u.Status,
		"created_at":    u.CreatedAt,
		"last_login_at": u.LastLoginAt,
	}
}

// publicHotel renders a hotel for the admin SPA.
func publicHotel(h *store.Hotel, kioskCount int) map[string]any {
	if kioskCount == 0 {
		kioskCount = h.KioskCount
	}
	return map[string]any{
		"id":          h.ID,
		"name":        h.Name,
		"pmsapi_url":  h.PmsApiURL,
		"notes":       h.Notes,
		"created_at":  h.CreatedAt,
		"updated_at":  h.UpdatedAt,
		"kiosk_count": kioskCount,
	}
}

func publicHotels(hs []store.Hotel) []map[string]any {
	out := make([]map[string]any, 0, len(hs))
	for i := range hs {
		out = append(out, publicHotel(&hs[i], hs[i].KioskCount))
	}
	return out
}

// adminKiosk returns the admin-facing JSON shape, including the device
// key (the admin SPA needs it for the copy-to-clipboard card).
func adminKiosk(k *store.Kiosk) map[string]any {
	return map[string]any{
		"id":                 k.ID,
		"hotel_id":           k.HotelID,
		"display_name":       k.DisplayName,
		"legacy_group_id":    k.LegacyGroupID,
		"legacy_group_label": k.LegacyGroupLabel,
		"theme":              k.Theme,
		"languages":          k.Languages,
		"device_key":         k.DeviceKey,
		"hero_image":         k.HeroImage,
		"logo":               k.Logo,
		"support_phone":      k.SupportPhone,
		"support_email":      k.SupportEmail,
		"status":             k.Status,
		"created_at":         k.CreatedAt,
		"updated_at":         k.UpdatedAt,
	}
}

func adminKioskList(ks []store.Kiosk) []map[string]any {
	out := make([]map[string]any, 0, len(ks))
	for i := range ks {
		out = append(out, adminKiosk(&ks[i]))
	}
	return out
}

func validateHotelInput(name, pmsURL string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("name is required")
	}
	if !validURL(pmsURL) {
		return errors.New("pmsapi_url must be a valid http(s) URL")
	}
	return nil
}

func validateKioskInput(displayName, theme string, langs []string) error {
	if strings.TrimSpace(displayName) == "" {
		return errors.New("display_name is required")
	}
	if strings.TrimSpace(theme) == "" {
		return errors.New("theme is required")
	}
	if len(langs) == 0 {
		return errors.New("at least one language is required")
	}
	return nil
}

func validURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return false
	}
	return true
}

func normaliseURL(raw string) string {
	return strings.TrimRight(strings.TrimSpace(raw), "/")
}
