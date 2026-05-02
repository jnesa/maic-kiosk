package domain

import "errors"

// Errors returned by ports. Adapters wrap transport-specific failures
// in these so handlers can map them to HTTP status codes without
// caring about the upstream's wire shape.
var (
	ErrNotFound          = errors.New("not found")
	ErrUnauthorized      = errors.New("unauthorized")
	ErrInvalidInput      = errors.New("invalid input")
	ErrUpstream          = errors.New("upstream error")
	ErrTenantMismatch    = errors.New("tenant mismatch")
)
