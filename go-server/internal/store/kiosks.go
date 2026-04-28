package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

// Kiosk is one check-in URL. The `id` is the public, opaque identifier
// (e.g. "k_8a3f9c1bf04a4d2faa8b7e1c2d3e4f01").
type Kiosk struct {
	ID                string
	HotelID           int64
	DisplayName       string
	LegacyGroupID     *int64
	LegacyGroupLabel  *string
	Theme             string
	Languages         []string
	DeviceKey         string
	HeroImage         *string
	Logo              *string
	SupportPhone      *string
	SupportEmail      *string
	Status            string // "active" | "disabled"
	CreatedAt         string
	UpdatedAt         string

	// Joined hotel fields, populated by FindKioskWithHotel for the proxy
	// fast path. Empty when the kiosk was loaded standalone.
	HotelName      string
	HotelPmsApiURL string
}

// ErrKioskNotFound — lookup miss.
var ErrKioskNotFound = errors.New("kiosk not found")

// KioskInput is the create/update DTO. Pointers mark optional fields.
type KioskInput struct {
	DisplayName      string
	LegacyGroupID    *int64
	LegacyGroupLabel string
	Theme            string
	Languages        []string
	HeroImage        string
	Logo             string
	SupportPhone     string
	SupportEmail     string
}

// CreateKiosk inserts a new kiosk under the given hotel. The caller
// generates `id` and `deviceKey` so the response can echo them in one
// round-trip without a re-fetch.
func (s *Store) CreateKiosk(ctx context.Context, id string, hotelID int64, deviceKey string, in KioskInput) (*Kiosk, error) {
	now := nowISO()
	langs, err := encodeLanguages(in.Languages)
	if err != nil {
		return nil, err
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO kiosks (
  id, hotel_id, display_name, legacy_group_id, legacy_group_label,
  theme, languages, device_key, hero_image, logo,
  support_phone, support_email, status, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'active', ?, ?)`,
		id, hotelID, in.DisplayName, in.LegacyGroupID, nullable(in.LegacyGroupLabel),
		in.Theme, langs, deviceKey, nullable(in.HeroImage), nullable(in.Logo),
		nullable(in.SupportPhone), nullable(in.SupportEmail), now, now)
	if err != nil {
		return nil, err
	}
	return s.FindKiosk(ctx, id)
}

// FindKiosk loads one kiosk by its public id.
func (s *Store) FindKiosk(ctx context.Context, id string) (*Kiosk, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, hotel_id, display_name, legacy_group_id, legacy_group_label,
       theme, languages, device_key, hero_image, logo,
       support_phone, support_email, status, created_at, updated_at
FROM kiosks WHERE id = ?`, id)
	return scanKiosk(row)
}

// FindKioskWithHotel powers the kiosk proxy fast path: it returns the
// kiosk plus the joined hotel fields the proxy needs (URL + name).
// Disabled kiosks are returned too — the caller decides how to respond.
func (s *Store) FindKioskWithHotel(ctx context.Context, id string) (*Kiosk, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT k.id, k.hotel_id, k.display_name, k.legacy_group_id, k.legacy_group_label,
       k.theme, k.languages, k.device_key, k.hero_image, k.logo,
       k.support_phone, k.support_email, k.status, k.created_at, k.updated_at,
       h.name, h.pmsapi_url
FROM kiosks k
JOIN hotels h ON h.id = k.hotel_id
WHERE k.id = ?`, id)
	var k Kiosk
	var legacyGroupID sql.NullInt64
	var legacyGroupLabel, heroImage, logo, supportPhone, supportEmail sql.NullString
	var langsJSON string
	if err := row.Scan(
		&k.ID, &k.HotelID, &k.DisplayName, &legacyGroupID, &legacyGroupLabel,
		&k.Theme, &langsJSON, &k.DeviceKey, &heroImage, &logo,
		&supportPhone, &supportEmail, &k.Status, &k.CreatedAt, &k.UpdatedAt,
		&k.HotelName, &k.HotelPmsApiURL,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrKioskNotFound
		}
		return nil, err
	}
	hydrateOptionalKioskFields(&k, legacyGroupID, legacyGroupLabel, heroImage, logo, supportPhone, supportEmail)
	if err := json.Unmarshal([]byte(langsJSON), &k.Languages); err != nil {
		return nil, err
	}
	return &k, nil
}

// ListKiosksByHotel returns every kiosk under one hotel, newest first.
func (s *Store) ListKiosksByHotel(ctx context.Context, hotelID int64) ([]Kiosk, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, hotel_id, display_name, legacy_group_id, legacy_group_label,
       theme, languages, device_key, hero_image, logo,
       support_phone, support_email, status, created_at, updated_at
FROM kiosks WHERE hotel_id = ? ORDER BY id DESC`, hotelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Kiosk
	for rows.Next() {
		k, err := scanKiosk(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *k)
	}
	return out, rows.Err()
}

// KioskPatch is the partial-update DTO for PATCH /kiosks/{id}. nil = no
// change. Empty string for an optional text field clears it.
type KioskPatch struct {
	DisplayName      *string
	LegacyGroupID    *int64
	LegacyGroupLabel *string
	Theme            *string
	Languages        []string // nil leaves unchanged; empty slice = no languages
	HeroImage        *string
	Logo             *string
	SupportPhone     *string
	SupportEmail     *string
}

// UpdateKiosk applies a partial patch.
func (s *Store) UpdateKiosk(ctx context.Context, id string, p KioskPatch) (*Kiosk, error) {
	k, err := s.FindKiosk(ctx, id)
	if err != nil {
		return nil, err
	}
	if p.DisplayName != nil {
		k.DisplayName = *p.DisplayName
	}
	if p.LegacyGroupID != nil {
		k.LegacyGroupID = p.LegacyGroupID
	}
	if p.LegacyGroupLabel != nil {
		if *p.LegacyGroupLabel == "" {
			k.LegacyGroupLabel = nil
		} else {
			v := *p.LegacyGroupLabel
			k.LegacyGroupLabel = &v
		}
	}
	if p.Theme != nil {
		k.Theme = *p.Theme
	}
	if p.Languages != nil {
		k.Languages = p.Languages
	}
	if p.HeroImage != nil {
		k.HeroImage = optionalText(*p.HeroImage)
	}
	if p.Logo != nil {
		k.Logo = optionalText(*p.Logo)
	}
	if p.SupportPhone != nil {
		k.SupportPhone = optionalText(*p.SupportPhone)
	}
	if p.SupportEmail != nil {
		k.SupportEmail = optionalText(*p.SupportEmail)
	}
	k.UpdatedAt = nowISO()
	langs, err := encodeLanguages(k.Languages)
	if err != nil {
		return nil, err
	}
	_, err = s.db.ExecContext(ctx, `
UPDATE kiosks SET
  display_name = ?, legacy_group_id = ?, legacy_group_label = ?,
  theme = ?, languages = ?, hero_image = ?, logo = ?,
  support_phone = ?, support_email = ?, updated_at = ?
WHERE id = ?`,
		k.DisplayName, k.LegacyGroupID, ptrToNullable(k.LegacyGroupLabel),
		k.Theme, langs, ptrToNullable(k.HeroImage), ptrToNullable(k.Logo),
		ptrToNullable(k.SupportPhone), ptrToNullable(k.SupportEmail), k.UpdatedAt, id)
	if err != nil {
		return nil, err
	}
	return k, nil
}

// RotateKioskKey replaces the device key. Caller generates the new
// secret so it can be returned to the client without an extra SELECT.
func (s *Store) RotateKioskKey(ctx context.Context, id, newKey string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE kiosks SET device_key = ?, updated_at = ? WHERE id = ?`,
		newKey, nowISO(), id)
	return err
}

// SetKioskStatus toggles between 'active' and 'disabled'.
func (s *Store) SetKioskStatus(ctx context.Context, id, status string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE kiosks SET status = ?, updated_at = ? WHERE id = ?`,
		status, nowISO(), id)
	return err
}

// DeleteKiosk hard-removes a kiosk. Audit-logged at the caller.
func (s *Store) DeleteKiosk(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM kiosks WHERE id = ?`, id)
	return err
}

func scanKiosk(row interface{ Scan(...any) error }) (*Kiosk, error) {
	var k Kiosk
	var legacyGroupID sql.NullInt64
	var legacyGroupLabel, heroImage, logo, supportPhone, supportEmail sql.NullString
	var langsJSON string
	if err := row.Scan(
		&k.ID, &k.HotelID, &k.DisplayName, &legacyGroupID, &legacyGroupLabel,
		&k.Theme, &langsJSON, &k.DeviceKey, &heroImage, &logo,
		&supportPhone, &supportEmail, &k.Status, &k.CreatedAt, &k.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrKioskNotFound
		}
		return nil, err
	}
	hydrateOptionalKioskFields(&k, legacyGroupID, legacyGroupLabel, heroImage, logo, supportPhone, supportEmail)
	if err := json.Unmarshal([]byte(langsJSON), &k.Languages); err != nil {
		return nil, err
	}
	return &k, nil
}

// hydrateOptionalKioskFields collapses the sql.Null* columns into the
// Kiosk's pointer fields. Pulled out because two scan paths use it.
func hydrateOptionalKioskFields(k *Kiosk, legacyGroupID sql.NullInt64, legacyGroupLabel, heroImage, logo, supportPhone, supportEmail sql.NullString) {
	if legacyGroupID.Valid {
		v := legacyGroupID.Int64
		k.LegacyGroupID = &v
	}
	if legacyGroupLabel.Valid {
		v := legacyGroupLabel.String
		k.LegacyGroupLabel = &v
	}
	if heroImage.Valid {
		v := heroImage.String
		k.HeroImage = &v
	}
	if logo.Valid {
		v := logo.String
		k.Logo = &v
	}
	if supportPhone.Valid {
		v := supportPhone.String
		k.SupportPhone = &v
	}
	if supportEmail.Valid {
		v := supportEmail.String
		k.SupportEmail = &v
	}
}

func encodeLanguages(langs []string) (string, error) {
	if langs == nil {
		langs = []string{}
	}
	b, err := json.Marshal(langs)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func optionalText(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func ptrToNullable(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}
