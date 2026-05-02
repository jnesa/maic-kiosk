package domain

// ReservationSummary is the lightweight view returned by reservation
// search — enough for the SPA to display a candidate list and let the
// guest pick the right reservation. Mirrors the legacy
// `kiosk_search_reservations` response row.
type ReservationSummary struct {
	ReservationID string `json:"reservation_id"`
	FirstName     string `json:"first_name"`
	LastName      string `json:"last_name"`
	Arrival       string `json:"arrival"`
	Departure     string `json:"departure"`
}

// GuestData is one guest on a reservation. Field names mirror the
// SPA's existing `GuestData` type (kiosk-spa/src/api/types.ts) so the
// SPA bundle can keep working unchanged. The legacymaichttp adapter
// translates between this and the legacy snake_case wire shape.
type GuestData struct {
	ID                 *int64 `json:"id"`
	Title              string `json:"title"`
	FName              string `json:"fname"`
	LName              string `json:"lname"`
	DOB                string `json:"dob"`
	Country            string `json:"country"`
	City               string `json:"city"`
	Postal             string `json:"postal"`
	Street             string `json:"street"`
	HouseNumber        string `json:"house_number"`
	Document           *int64 `json:"document"`
	DocumentID         string `json:"document_id"`
	DocumentIssuer     string `json:"document_issuer"`
	DocumentIssueDate  string `json:"document_issue_date"`
	Nationality        string `json:"nationality"`
	Phone              string `json:"phone"`
	TraveltimeChanged  bool   `json:"traveltime_changed"`
	TraveltimeArrival  string `json:"traveltime_arrival"`
	TraveltimeDeparture string `json:"traveltime_departure"`
	SpecialTravel      bool   `json:"specialtravel"`
	SpecialTravelEvent string `json:"special_travel_event_id"`
	BusinessTravel     bool   `json:"businesstravel"`
	AnnualCard         bool   `json:"annualcard"`
	AnnualCardNumber   string `json:"annualcard_number"`
	Handicap           bool   `json:"handicap"`
	HandicapNeedHelp   bool   `json:"handicap_needhelp"`
	HandicapNumber     string `json:"handicap_number"`
	HandicapIsHelp     bool   `json:"handicap_is_help"`
}

// FirmData is the firm/billing block the guest fills in once per
// reservation. Field names mirror the SPA's existing `FirmData` type
// (kiosk-spa/src/api/types.ts).
type FirmData struct {
	CompName              string `json:"compname"`
	VatID                 string `json:"vatid"`
	Address               string `json:"address"`
	City                  string `json:"city"`
	Arrival               string `json:"arrival"`
	ArrivalVia            string `json:"arrival_via"`
	ArrivalWithCar        bool   `json:"arrival_with_car"`
	Phone                 string `json:"phone"`
	Email                 string `json:"email"`
	UseFirmForBilling     bool   `json:"useFirmForBilling"`
	UseAnotherBillingAddr bool   `json:"useAnotherBillingAddress"`
	BillingAddress        string `json:"billing_address"`
	Transfer              bool   `json:"transfer"`
	TransferText          string `json:"transferText"`
	BabyBed               bool   `json:"babyBed"`
	BabyBedText           string `json:"babyBedText"`
	DogPackage            bool   `json:"dogPackage"`
	DogPackageText        string `json:"dogPackageText"`
	Allergies             bool   `json:"alergies"`
	AllergiesText         string `json:"alergiesText"`
	Accessible            bool   `json:"accessible"`
	AdditionalLinens      bool   `json:"additionalLinens"`
	AdditionalLinensCount string `json:"additionalLinensAmount"`
	PreferredCommunication string `json:"preferedCommunication"`
	Signature             string `json:"signature"`
}

// PrestayConfig is the per-property form configuration returned by
// `kiosk_prestay_form`. It's a free-form JSON blob whose shape comes
// from the legacy app's `app_settings.prestay_form` column (or the
// per-group override in `app_setting_group_groups`). The kiosk Go
// service treats it as opaque and forwards it to the SPA verbatim.
type PrestayConfig map[string]any
