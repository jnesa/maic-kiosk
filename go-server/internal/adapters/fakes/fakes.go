// Package fakes holds in-memory port implementations used by handler
// and integration tests. Each fake is the simplest thing that
// satisfies its port interface — no concurrency safety, no
// persistence beyond a single test.
package fakes

import (
	"context"
	"sync"

	"github.com/maic/checkin-kiosk-api/internal/domain"
)

// OperatorAuth fake — accepts a configured username/password and
// returns a synthesised operator with a deterministic token.
type OperatorAuth struct {
	mu        sync.Mutex
	Allow     map[string]string // username → password
	NextToken string

	LastUser, LastDeviceID string
}

func NewOperatorAuth() *OperatorAuth {
	return &OperatorAuth{Allow: map[string]string{"alice": "wonderland"}, NextToken: "fake-token"}
}

func (f *OperatorAuth) Login(_ context.Context, username, password, deviceID string) (*domain.LoginResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.LastUser = username
	f.LastDeviceID = deviceID

	if pw, ok := f.Allow[username]; !ok || pw != password {
		return nil, domain.ErrUnauthorized
	}
	return &domain.LoginResult{Operator: domain.Operator{
		UserID:             7,
		Username:           username,
		Email:              username + "@example.com",
		Name:               username,
		LegacySessionToken: f.NextToken,
	}}, nil
}

// KioskStore fake.
type KioskStore struct {
	mu     sync.Mutex
	Kiosks []domain.Kiosk
}

func NewKioskStore(seed ...domain.Kiosk) *KioskStore {
	if len(seed) == 0 {
		seed = []domain.Kiosk{
			{ID: 1, Name: "Lobby", UUID: "kiosk-uuid-1", GroupID: 11},
			{ID: 2, Name: "Spa", UUID: "kiosk-uuid-2", GroupID: 11},
		}
	}
	return &KioskStore{Kiosks: append([]domain.Kiosk(nil), seed...)}
}

func (f *KioskStore) ListForUser(_ context.Context, _ domain.Operator) ([]domain.Kiosk, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]domain.Kiosk, len(f.Kiosks))
	copy(out, f.Kiosks)
	return out, nil
}

func (f *KioskStore) GetByUUID(_ context.Context, uuid string) (*domain.Kiosk, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, k := range f.Kiosks {
		if k.UUID == uuid {
			cp := k
			return &cp, nil
		}
	}
	return nil, domain.ErrNotFound
}

// Reservations fake. Records each call so handler tests can assert
// the right port was hit with the right inputs.
type Reservations struct {
	mu sync.Mutex

	SearchResults map[string][]domain.ReservationSummary
	FormConfig    domain.PrestayConfig

	SaveGuestCalls    []SaveGuestCall
	SaveFirmCalls     []SaveFirmCall
	DeleteGuestCalls  []DeleteGuestCall
	SaveFormCalls     []SaveFormCall
	NextSavedGuestID  int64
	SaveFormError     error
}

type SaveGuestCall struct {
	UUID, ReservationID string
	Guest               domain.GuestData
}

type SaveFirmCall struct {
	UUID, ReservationID string
	Firm                domain.FirmData
}

type DeleteGuestCall struct {
	UUID, ReservationID string
	GuestID             int64
}

type SaveFormCall struct {
	UUID, ReservationID string
	Payload             map[string]any
}

func NewReservations() *Reservations {
	return &Reservations{
		SearchResults: map[string][]domain.ReservationSummary{
			"Pieper": {{ReservationID: "ABC-1", FirstName: "Carmen", LastName: "Pieper", Arrival: "2026-11-20", Departure: "2026-11-23"}},
		},
		FormConfig:       domain.PrestayConfig{"useMRZ": true},
		NextSavedGuestID: 100,
	}
}

func (f *Reservations) Search(_ context.Context, kioskUUID, search string) ([]domain.ReservationSummary, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if rs, ok := f.SearchResults[search]; ok {
		return rs, nil
	}
	return []domain.ReservationSummary{}, nil
}

func (f *Reservations) PrestayForm(_ context.Context, _ string) (domain.PrestayConfig, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.FormConfig, nil
}

func (f *Reservations) SaveGuest(_ context.Context, kioskUUID, reservationID string, guest domain.GuestData, _ *domain.Operator) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.SaveGuestCalls = append(f.SaveGuestCalls, SaveGuestCall{kioskUUID, reservationID, guest})
	id := f.NextSavedGuestID
	f.NextSavedGuestID++
	return id, nil
}

func (f *Reservations) SaveFirm(_ context.Context, kioskUUID, reservationID string, firm domain.FirmData, _ *domain.Operator) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.SaveFirmCalls = append(f.SaveFirmCalls, SaveFirmCall{kioskUUID, reservationID, firm})
	return nil
}

func (f *Reservations) DeleteGuest(_ context.Context, kioskUUID, reservationID string, guestID int64, _ *domain.Operator) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.DeleteGuestCalls = append(f.DeleteGuestCalls, DeleteGuestCall{kioskUUID, reservationID, guestID})
	return nil
}

func (f *Reservations) SaveForm(_ context.Context, kioskUUID, reservationID string, payload map[string]any, _ *domain.Operator) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.SaveFormCalls = append(f.SaveFormCalls, SaveFormCall{kioskUUID, reservationID, payload})
	return f.SaveFormError
}

// Feratel fake.
type Feratel struct {
	mu       sync.Mutex
	Mappings map[string]map[string]any // postal → response
}

func NewFeratel() *Feratel {
	return &Feratel{
		Mappings: map[string]map[string]any{
			"1010": {"success": true, "city": "Wien"},
			"":     {"success": false},
		},
	}
}

func (f *Feratel) FetchCityFromPostal(_ context.Context, _, _, postal, _ string) (map[string]any, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if v, ok := f.Mappings[postal]; ok {
		return v, nil
	}
	return map[string]any{"success": false}, nil
}
