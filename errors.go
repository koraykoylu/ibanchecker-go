package ibanchecker

import (
	"errors"
	"fmt"
)

// Sentinel errors for the failures a caller usually branches on. Compare them
// with errors.Is; reach for errors.As and *Error when you need the status, the
// machine-readable code or the decoded body.
var (
	// ErrBadRequest is returned for HTTP 400: the request was malformed, or a
	// trial call went over the trial size (Code "TOO_MANY_IBANS" for
	// ValidateBulk, "TEXT_TOO_LONG" for Extract).
	ErrBadRequest = errors.New("bad request")

	// ErrAuthentication is returned for HTTP 401: the API key is missing,
	// invalid or inactive. Every method except CountryFormat needs a key, so
	// a LookupBIC call without one also gets it from the API's 401.
	ErrAuthentication = errors.New("authentication failed")

	// ErrNotFound is returned for HTTP 404: no such country code or BIC.
	ErrNotFound = errors.New("not found")

	// ErrRateLimit is returned for HTTP 429: the key's monthly quota was
	// used up, or the call costs more than the requests left this month
	// (Code "QUOTA_EXCEEDED"; ValidateBulk counts one request per IBAN and
	// Extract one per IBAN found), or a CountryFormat call without a key went
	// over 100 requests an hour per IP (Code "RATE_LIMIT_EXCEEDED"). The
	// hourly limit applies to CountryFormat only.
	ErrRateLimit = errors.New("rate limited")

	// ErrAPI is returned for any other error status, and for a response body
	// that could not be read as JSON. That includes HTTP 403 with Code
	// "PLAN_REQUIRED", a call the key's plan does not include; Response then
	// carries "required_plan" ("basic" or "growth") and "upgrade_url".
	ErrAPI = errors.New("api error")

	// ErrTransport is returned when the request never reached the API: DNS,
	// TLS, connection or timeout.
	ErrTransport = errors.New("transport error")
)

// Error carries everything the API said about a failure.
//
// A malformed IBAN is not an error: Validate returns a ValidationResult with
// Valid false. These are returned for transport, authentication, plan, quota
// and server-side problems only.
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
