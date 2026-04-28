package store

import (
	"context"
	"database/sql"
	"errors"
)

// Hotel maps 1:1 to a legacy PMSApi subdomain.
type Hotel struct {
	ID         int64
	Name       string
	PmsApiURL  string
	Notes      *string
	CreatedAt  string
	UpdatedAt  string
	KioskCount int // populated by ListHotels; zero when fetched directly
}

// ErrHotelNotFound is returned by lookups on miss.
var ErrHotelNotFound = errors.New("hotel not found")

// CreateHotel inserts a new hotel row.
func (s *Store) CreateHotel(ctx context.Context, name, pmsapiURL, notes string) (*Hotel, error) {
	now := nowISO()
	res, err := s.db.ExecContext(ctx, `
INSERT INTO hotels (name, pmsapi_url, notes, created_at, updated_at)
VALUES (?, ?, ?, ?, ?)`, name, pmsapiURL, nullable(notes), now, now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	h := &Hotel{ID: id, Name: name, PmsApiURL: pmsapiURL, CreatedAt: now, UpdatedAt: now}
	if notes != "" {
		h.Notes = &notes
	}
	return h, nil
}

// FindHotel returns a single hotel by id.
func (s *Store) FindHotel(ctx context.Context, id int64) (*Hotel, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, name, pmsapi_url, notes, created_at, updated_at
FROM hotels WHERE id = ?`, id)
	return scanHotel(row)
}

// ListHotels returns every hotel with the kiosk count joined in.
func (s *Store) ListHotels(ctx context.Context) ([]Hotel, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT h.id, h.name, h.pmsapi_url, h.notes, h.created_at, h.updated_at,
       (SELECT COUNT(*) FROM kiosks k WHERE k.hotel_id = h.id) AS kiosk_count
FROM hotels h
ORDER BY h.id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Hotel
	for rows.Next() {
		var h Hotel
		var notes sql.NullString
		if err := rows.Scan(&h.ID, &h.Name, &h.PmsApiURL, &notes, &h.CreatedAt, &h.UpdatedAt, &h.KioskCount); err != nil {
			return nil, err
		}
		if notes.Valid {
			v := notes.String
			h.Notes = &v
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// HotelPatch is the optional-fields struct for PATCH /hotels/{id}.
// nil pointer = leave unchanged; empty string for `Notes` clears the field.
type HotelPatch struct {
	Name      *string
	PmsApiURL *string
	Notes     *string
}

// UpdateHotel applies a partial patch and returns the row's new state.
func (s *Store) UpdateHotel(ctx context.Context, id int64, p HotelPatch) (*Hotel, error) {
	h, err := s.FindHotel(ctx, id)
	if err != nil {
		return nil, err
	}
	if p.Name != nil {
		h.Name = *p.Name
	}
	if p.PmsApiURL != nil {
		h.PmsApiURL = *p.PmsApiURL
	}
	var notesArg any
	if p.Notes != nil {
		if *p.Notes == "" {
			notesArg = nil
			h.Notes = nil
		} else {
			notesArg = *p.Notes
			n := *p.Notes
			h.Notes = &n
		}
	} else {
		notesArg = nullable(strDeref(h.Notes))
	}
	h.UpdatedAt = nowISO()
	_, err = s.db.ExecContext(ctx, `
UPDATE hotels SET name = ?, pmsapi_url = ?, notes = ?, updated_at = ?
WHERE id = ?`, h.Name, h.PmsApiURL, notesArg, h.UpdatedAt, id)
	if err != nil {
		return nil, err
	}
	return h, nil
}

// DeleteHotel cascade-removes the hotel and its kiosks. Caller is
// responsible for audit-logging the action.
func (s *Store) DeleteHotel(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM hotels WHERE id = ?`, id)
	return err
}

func scanHotel(row interface{ Scan(...any) error }) (*Hotel, error) {
	var h Hotel
	var notes sql.NullString
	if err := row.Scan(&h.ID, &h.Name, &h.PmsApiURL, &notes, &h.CreatedAt, &h.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrHotelNotFound
		}
		return nil, err
	}
	if notes.Valid {
		v := notes.String
		h.Notes = &v
	}
	return &h, nil
}

func strDeref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
