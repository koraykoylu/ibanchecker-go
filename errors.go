package ibanchecker

import (
	"errors"
	"fmt"
)

// Sentinel errors for the failures a caller usually branches on. Compare them
// with errors.Is; reach for errors.As and *Error when you need the status, the
// machine-readable code or the decoded body.
var (
	// ErrBadRequest is returned for HTTP 400: the request was malformed.
	ErrBadRequest = errors.New("bad request")

	// ErrAuthentication is returned for HTTP 401: the API key is missing,
	// invalid or inactive.
	ErrAuthentication = errors.New("authentication failed")

	// ErrNotFound is returned for HTTP 404: no such country code or BIC.
	ErrNotFound = errors.New("not found")

	// ErrRateLimit is returned for HTTP 429: the hourly rate limit or the
	// monthly quota was exceeded.
	ErrRateLimit = errors.New("rate limited")

	// ErrAPI is returned for any other error status, and for a response body
	// that could not be read as JSON.
	ErrAPI = errors.New("api error")

	// ErrTransport is returned when the request never reached the API: DNS,
	// TLS, connection or timeout.
	ErrTransport = errors.New("transport error")
)

// Error carries everything the API said about a failure.
//
// A malformed IBAN is not an error: Validate returns a ValidationResult with
// Valid false. These are returned for transport, authentication, quota and
// server-side problems only.
type Error struct {
	// Status is the HTTP status, or 0 when the request never completed.
	Status int

	// Code is the machine-readable code from the API body, for example
	// "BIC_NOT_FOUND". Empty when the API sent none.
	Code string

	// Message is the human-readable reason.
	Message string

	// Response is the decoded response body, when the API sent one.
	Response map[string]any

	// kind is the sentinel this error unwraps to.
	kind error
}

func (e *Error) Error() string {
	if e.Status > 0 {
		return fmt.Sprintf("ibanchecker: %s (HTTP %d)", e.Message, e.Status)
	}
	return fmt.Sprintf("ibanchecker: %s", e.Message)
}

// Unwrap reports the sentinel, so errors.Is(err, ErrNotFound) works.
func (e *Error) Unwrap() error { return e.kind }
