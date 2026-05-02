// Package kiosk owns /api/kiosk/v1/* — the public, UUID-routed
// guest-flow surface. Every endpoint takes the kiosk UUID from the
// URL and forwards to the legacy MAIC endpoint via the Reservations
// port.
//
// No auth on the guest path. Knowing the UUID is the entire
// authorisation surface; the upstream legacy controller scopes by
// kiosk.group_id when looking up reservations.
package kiosk

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/maic/checkin-kiosk-api/internal/domain"
	"github.com/maic/checkin-kiosk-api/internal/ports"
	"github.com/maic/checkin-kiosk-api/internal/session"
)

// Handler bundles the dependencies kiosk routes need.
type Handler struct {
	Reservations ports.Reservations
	Feratel      ports.Feratel
	Kiosks       ports.KioskStore
}

// New wires a Handler.
func New(res ports.Reservations, feratel ports.Feratel, ks ports.KioskStore) *Handler {
	return &Handler{Reservations: res, Feratel: feratel, Kiosks: ks}
}

// Mount attaches every kiosk route. Health/ready are global; the
// guest-flow group lives under /{uuid}.
func (h *Handler) Mount(r chi.Router) {
	r.Get("/health", h.health)
	r.Get("/ready", h.ready)

	r.Route("/{uuid}", func(r chi.Router) {
		r.Get("/config", h.config)

		// Reservation flow
		r.Post("/lookup", h.lookup)
		r.Post("/select", h.selectReservation)
		r.Post("/form", h.form)
		r.Post("/save-guest", h.saveGuest)
		r.Post("/save-firm", h.saveFirm)
		r.Post("/delete-guest", h.deleteGuest)
		r.Post("/submit", h.submit)

		// Helpers
		r.Post("/postal-lookup", h.postalLookup)
	})
}

// --- health/ready ---

func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "healthy", "service": "checkin-kiosk-api"})
}

func (h *Handler) ready(w http.ResponseWriter, _ *http.Request) {
	// In phase 1 we have no persistent state to ping — readiness ==
	// health. If we ever add a Valkey check (phase 2), it goes here.
	writeJSON(w, http.StatusOK, map[string]any{"status": "ready"})
}

// --- config ---

// config returns the public, sanitised view of a kiosk. The legacy
// /api/get_users_kiosks doesn't allow anonymous lookup, so we
// validate the UUID by attempting to fetch the prestay form (which is
// public per the legacy controller). On success we return a minimal
// shape; the SPA gets the full prestay config from /form on the next
// hop.
func (h *Handler) config(w http.ResponseWriter, r *http.Request) {
	uuid := chi.URLParam(r, "uuid")
	k, err := h.Kiosks.GetByUUID(r.Context(), uuid)
	if err != nil {
		// `kiosk_prestay_form` returns admin.error for unknown UUIDs;
		// we surface that as 410 Gone so the SPA renders "deactivated".
		if errors.Is(err, domain.ErrUpstream) || errors.Is(err, domain.ErrNotFound) {
			writeErr(w, http.StatusGone, "kiosk_disabled", "this kiosk is not currently active")
			return
		}
		writeUpstreamErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"uuid": k.UUID,
		"name": k.Name,
	})
}

// --- guest flow ---

type lookupReq struct {
	// SPA can send any of these — adapter normalises via a single
	// `search` field upstream.
	Search        string `json:"search"`
	LastName      string `json:"lastName"`
	ReservationID string `json:"reservationId"`
	ArrivalDate   string `json:"arrivalDate"`
}

// lookup translates the legacy `kiosk_search_reservations` array
// response into the discriminated-union shape the SPA expects:
//
//   { result: 'matched',   token, expiresAt, reservation } when N=1
//   { result: 'ambiguous', candidateToken, candidates: [...] } when N>1
//   { result: 'not_found' } when N=0
//
// Legacy newDev has no HMAC `candidateToken`/`token` — we use the
// reservation_id string as both. The SPA treats them opaquely; later
// hops (/select, /form) just need the reservation_id, which they
// can carry as well.
func (h *Handler) lookup(w http.ResponseWriter, r *http.Request) {
	uuid := chi.URLParam(r, "uuid")
	var req lookupReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request", "invalid JSON")
		return
	}
	q := req.Search
	if q == "" {
		q = req.LastName
	}
	if q == "" {
		q = req.ReservationID
	}
	if q == "" {
		writeErr(w, http.StatusBadRequest, "bad_request", "search term required")
		return
	}

	matches, err := h.Reservations.Search(r.Context(), uuid, q)
	if err != nil {
		writeUpstreamErr(w, err)
		return
	}

	switch len(matches) {
	case 0:
		writeJSON(w, http.StatusOK, map[string]any{"result": "not_found"})
	case 1:
		writeJSON(w, http.StatusOK, map[string]any{
			"result":      "matched",
			"token":       matches[0].ReservationID, // opaque to SPA
			"expiresAt":   "",
			"reservation": reservationView(matches[0]),
		})
	default:
		cands := make([]map[string]any, 0, len(matches))
		for _, m := range matches {
			cands = append(cands, map[string]any{
				"candidateId": m.ReservationID,
				"firstName":   m.FirstName,
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"result":         "ambiguous",
			"candidateToken": "", // legacy newDev has no token
			"candidates":     cands,
		})
	}
}

type selectReq struct {
	CandidateToken string `json:"candidateToken"`
	CandidateID    string `json:"candidateId"`
	// Some SPA flows still send reservationId — accept it.
	ReservationID string `json:"reservationId"`
}

// selectReservation lets the SPA confirm a candidate from an ambiguous
// lookup result. The legacy app has no dedicated select endpoint
// (search returns rows directly), so we re-fetch the reservation by
// re-running the search with the reservation_id as the search term
// and finding the exact match — that gives us a full ReservationSummary
// to return in the matched shape.
//
// If the SPA passes us a reservationId directly (some paths skip the
// candidate-token dance), we use that.
func (h *Handler) selectReservation(w http.ResponseWriter, r *http.Request) {
	uuid := chi.URLParam(r, "uuid")
	var req selectReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request", "invalid JSON")
		return
	}
	resID := req.CandidateID
	if resID == "" {
		resID = req.ReservationID
	}
	if resID == "" {
		writeErr(w, http.StatusBadRequest, "bad_request", "candidateId or reservationId required")
		return
	}

	// Resolve the full reservation summary by searching for the
	// reservation_id (legacy `kiosk_search_reservations` matches against
	// reservation_id too, so this works).
	matches, err := h.Reservations.Search(r.Context(), uuid, resID)
	if err != nil {
		writeUpstreamErr(w, err)
		return
	}
	for _, m := range matches {
		if m.ReservationID == resID {
			writeJSON(w, http.StatusOK, map[string]any{
				"result":      "matched",
				"token":       m.ReservationID,
				"expiresAt":   "",
				"reservation": reservationView(m),
			})
			return
		}
	}
	writeErr(w, http.StatusNotFound, "not_found", "reservation not found for this kiosk")
}

// reservationView maps the lightweight legacy summary onto the SPA's
// ReservationSummary shape (kiosk-spa/src/api/types.ts).
//
// Fields the legacy doesn't return at this stage (id, adults,
// children, roomName, groupId, prestayDone) get safe defaults; the
// SPA fetches the full per-reservation data on /form next.
func reservationView(r domain.ReservationSummary) map[string]any {
	return map[string]any{
		"id":          0,
		"code":        r.ReservationID,
		"firstName":   r.FirstName,
		"lastName":    r.LastName,
		"arrival":     r.Arrival,
		"departure":   r.Departure,
		"adults":      1,
		"children":    0,
		"prestayDone": false,
	}
}

type formReq struct {
	ReservationID string `json:"reservationId"`
}

func (h *Handler) form(w http.ResponseWriter, r *http.Request) {
	uuid := chi.URLParam(r, "uuid")
	cfg, err := h.Reservations.PrestayForm(r.Context(), uuid)
	if err != nil {
		writeUpstreamErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"prestay_form": cfg})
}

type saveGuestReq struct {
	ReservationID string           `json:"reservationId"`
	Guest         domain.GuestData `json:"guest"`
}

func (h *Handler) saveGuest(w http.ResponseWriter, r *http.Request) {
	uuid := chi.URLParam(r, "uuid")
	var req saveGuestReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request", "invalid JSON")
		return
	}
	if req.ReservationID == "" {
		writeErr(w, http.StatusBadRequest, "bad_request", "reservationId required")
		return
	}
	op := session.FromContext(r.Context())
	id, err := h.Reservations.SaveGuest(r.Context(), uuid, req.ReservationID, req.Guest, op)
	if err != nil {
		writeUpstreamErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id})
}

type saveFirmReq struct {
	ReservationID string          `json:"reservationId"`
	Firm          domain.FirmData `json:"firm"`
}

func (h *Handler) saveFirm(w http.ResponseWriter, r *http.Request) {
	uuid := chi.URLParam(r, "uuid")
	var req saveFirmReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request", "invalid JSON")
		return
	}
	if req.ReservationID == "" {
		writeErr(w, http.StatusBadRequest, "bad_request", "reservationId required")
		return
	}
	op := session.FromContext(r.Context())
	if err := h.Reservations.SaveFirm(r.Context(), uuid, req.ReservationID, req.Firm, op); err != nil {
		writeUpstreamErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

type deleteGuestReq struct {
	ReservationID string `json:"reservationId"`
	GuestID       int64  `json:"guestId"`
}

func (h *Handler) deleteGuest(w http.ResponseWriter, r *http.Request) {
	uuid := chi.URLParam(r, "uuid")
	var req deleteGuestReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request", "invalid JSON")
		return
	}
	if req.ReservationID == "" || req.GuestID == 0 {
		writeErr(w, http.StatusBadRequest, "bad_request", "reservationId and guestId required")
		return
	}
	op := session.FromContext(r.Context())
	if err := h.Reservations.DeleteGuest(r.Context(), uuid, req.ReservationID, req.GuestID, op); err != nil {
		writeUpstreamErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// submit is the SPA's "I'm done with the form" call. The legacy
// app has no dedicated submit endpoint (newDev's routes/api.php has
// no kiosk_save_form route — see audit §3.5.4 Q1). For now we accept
// the request, do nothing upstream, and return 200 — the prior
// save-guest/save-firm calls have already persisted everything.
//
// If the legacy app later adds /api/kiosk_save_form, swap in
// h.Reservations.SaveForm(...) here.
func (h *Handler) submit(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// --- helpers ---

type postalReq struct {
	ReservationID string `json:"reservationId"`
	Postal        string `json:"postal"`
	Country       string `json:"country"`
}

func (h *Handler) postalLookup(w http.ResponseWriter, r *http.Request) {
	uuid := chi.URLParam(r, "uuid")
	var req postalReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request", "invalid JSON")
		return
	}
	if req.Postal == "" {
		writeErr(w, http.StatusBadRequest, "bad_request", "postal required")
		return
	}
	out, err := h.Feratel.FetchCityFromPostal(r.Context(), uuid, req.ReservationID, req.Postal, req.Country)
	if err != nil {
		writeUpstreamErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
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
