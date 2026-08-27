// Package apperr defines application-level error types and helpers.
package apperr

import "errors"

var (
	ErrNotFound        = errors.New("not found")
	ErrForbidden       = errors.New("forbidden")
	ErrConflict        = errors.New("conflict")
	ErrInvalidInput    = errors.New("invalid input")
	ErrInternal        = errors.New("internal error")
	ErrVariantNotReady = errors.New("variant not ready")
)
