// Package admin holds /api/admin/v1/* HTTP handlers. They are thin —
// every operation delegates to a port (OperatorAuth, KioskStore) so
// the same code works against any adapter (legacymaichttp today,
// Redis Streams or direct DB tomorrow).
package admin

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/maic/checkin-kiosk-api/internal/domain"
	"github.com/maic/checkin-kiosk-api/internal/ports"
	"github.com/maic/checkin-kiosk-api/internal/session"
)

// Handler bundles the dependencies admin routes need.
type Handler struct {
	Auth     ports.OperatorAuth
	Kiosks   ports.KioskStore
	Sessions *session.Manager
}

// New wires a Handler.
func New(auth ports.OperatorAuth, kiosks ports.KioskStore, sm *session.Manager) *Handler {
	return &Handler{Auth: auth, Kiosks: kiosks, Sessions: sm}
}

// Mount attaches admin routes onto the supplied chi router.
//
// /auth/login is public; everything else lives under
// session.RequireOperator.
func (h *Handler) Mount(r chi.Router) {
	r.Post("/auth/login", h.login)

	r.Group(func(r chi.Router) {
		r.Use(session.RequireOperator)

		r.Post("/auth/logout", h.logout)
		r.Get("/me", h.me)

		r.Get("/kiosks", h.listKiosks)
		r.Get("/kiosks/{uuid}", h.getKiosk)
	})
}

// --- handlers ---

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
	DeviceID string `json:"device_id"`
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request", "invalid JSON")
		return
	}
	if req.Username == "" || req.Password == "" {
		writeErr(w, http.StatusBadRequest, "bad_request", "username and password required")
		return
	}

	res, err := h.Auth.Login(r.Context(), req.Username, req.Password, req.DeviceID)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "invalid_credentials", "wrong username or password")
		return
	}
	if err := h.Sessions.Issue(w, res.Operator); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": publicOperator(res.Operator)})
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	h.Sessions.Clear(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	op := session.FromContext(r.Context())
	writeJSON(w, http.StatusOK, publicOperator(*op))
}

func (h *Handler) listKiosks(w http.ResponseWriter, r *http.Request) {
	op := session.FromContext(r.Context())
	kiosks, err := h.Kiosks.ListForUser(r.Context(), *op)
	if err != nil {
		writeUpstreamErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"kiosks": kiosks})
}

func (h *Handler) getKiosk(w http.ResponseWriter, r *http.Request) {
	uuid := chi.URLParam(r, "uuid")
	k, err := h.Kiosks.GetByUUID(r.Context(), uuid)
	if err != nil {
		writeUpstreamErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, k)
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": msg}})
}

// writeUpstreamErr maps domain errors to HTTP status codes.
func writeUpstreamErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrUnauthorized):
		writeErr(w, http.StatusUnauthorized, "unauthenticated", err.Error())
	case errors.Is(err, domain.ErrNotFound):
		writeErr(w, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, domain.ErrInvalidInput):
		writeErr(w, http.StatusBadRequest, "bad_request", err.Error())
	case errors.Is(err, domain.ErrTenantMismatch):
		writeErr(w, http.StatusForbidden, "tenant_mismatch", err.Error())
	default:
		writeErr(w, http.StatusBadGateway, "upstream_error", err.Error())
	}
}

func publicOperator(op domain.Operator) map[string]any {
	return map[string]any{
		"user_id":  op.UserID,
		"username": op.Username,
		"email":    op.Email,
		"name":     op.Name,
	}
}
