package legacymaichttp

import "context"

// FetchCityFromPostal forwards to POST /api/kiosk_fetch_city_from_postal
// {reservation_id, postal, country}. The legacy controller delegates
// to FeratelController::fetchCityFromPostal.
//
// Response shape varies by Feratel config — sometimes
// {success:true, city:"...", country_code:"AT", ...}, sometimes
// {success:false} when the postal isn't found. We pass it through
// as a generic map so the SPA can decide.
func (c *Client) FetchCityFromPostal(ctx context.Context, kioskUUID, reservationID, postal, country string) (map[string]any, error) {
	body := map[string]any{
		"reservation_id": reservationID,
		"postal":         postal,
		"country":        country,
		"uuid":           kioskUUID,
	}
	var resp map[string]any
	if err := c.postJSON(ctx, "/kiosk_fetch_city_from_postal", body, &resp, nil); err != nil {
		return nil, err
	}
	return resp, nil
}
