// Package ibanchecker is the official Go client for the ibanchecker.cash IBAN
// validation API.
//
// Validate IBANs across 92 countries, validate up to 100 IBANs per request,
// extract IBANs from free text, look up country format specifications and
// resolve SWIFT/BIC codes.
//
// An API key is optional. Without one, requests are limited to 100 per hour
// per IP. Get a free key at https://ibanchecker.cash/api-docs.
//
//	client := ibanchecker.New("") // or ibanchecker.New("iban_your_key")
//
//	result, err := client.Validate(context.Background(), "DE89 3704 0044 0532 0130 00")
//	if err != nil {
//		log.Fatal(err)
//	}
//	if result.Valid {
//		fmt.Println(result.BankName, result.BIC)
//	}
//
// A malformed IBAN is not an error: Validate returns a ValidationResult with
// Valid false and an Error plus ErrorCode explaining why. Errors are returned
// for transport, authentication, quota and server-side problems only.
package ibanchecker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Version goes out in the User-Agent header.
const Version = "0.1.0"

// DefaultBaseURL is the production API.
const DefaultBaseURL = "https://ibanchecker.cash/api/v1"

// Client talks to the ibanchecker.cash API. The zero value is not usable; call
// New. A Client is safe for concurrent use by multiple goroutines.
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL points the client at a different host. Any trailing slash is
// trimmed.
func WithBaseURL(baseURL string) Option {
	return func(c *Client) { c.baseURL = strings.TrimRight(baseURL, "/") }
}

// WithHTTPClient replaces the HTTP client, which is how an application routes
// these calls through its own stack, and how the tests run against an
// httptest server. It also overrides WithTimeout and the redirect policy
// below, so set both on the client you pass in.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.httpClient = hc }
}

// WithTimeout sets the timeout on the default HTTP client.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.httpClient.Timeout = d }
}

// New returns a Client. Pass an empty apiKey for unauthenticated use.
func New(apiKey string, opts ...Option) *Client {
	c := &Client{
		apiKey:  apiKey,
		baseURL: DefaultBaseURL,
		httpClient: &http.Client{
			Timeout:       10 * time.Second,
			CheckRedirect: refuseRedirect,
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// refuseRedirect stops rather than follows, and says where the redirect led.
//
// Following would be worse than it sounds. net/http turns a POST into a GET on
// a 301, which is what the RFC asks for, so an http base URL would silently
// reach the https endpoint as a GET and come back 405 Method Not Allowed, with
// nothing in the message to explain it. And a redirect to another host is how
// an API key travels somewhere it was never meant to go.
func refuseRedirect(req *http.Request, _ []*http.Request) error {
	return fmt.Errorf("refusing to follow the redirect to %s: set the base URL to that address (the API is https, and an http base URL redirects)", req.URL)
}

// Validate validates a single IBAN.
//
// A malformed IBAN is not an error: the result comes back with Valid false and
// an Error plus ErrorCode explaining why.
func (c *Client) Validate(ctx context.Context, iban string) (*ValidationResult, error) {
	var out ValidationResult
	err := c.request(ctx, http.MethodPost, "/validate", map[string]any{"iban": iban}, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// ValidateBulk validates up to 100 IBANs in one request. Results come back in
// the same order as the input.
func (c *Client) ValidateBulk(ctx context.Context, ibans []string) (*BatchResult, error) {
	if ibans == nil {
		ibans = []string{}
	}
	var out BatchResult
	err := c.request(ctx, http.MethodPost, "/validate/bulk", map[string]any{"ibans": ibans}, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// Extract scans free text (emails, invoices) for IBAN-shaped strings and
// validates each candidate. Up to 50,000 characters per request.
func (c *Client) Extract(ctx context.Context, text string) (*BatchResult, error) {
	var out BatchResult
	err := c.request(ctx, http.MethodPost, "/extract", map[string]any{"text": text}, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// CountryFormat returns the IBAN format specification for an ISO 3166-1
// alpha-2 country code, for example "DE".
func (c *Client) CountryFormat(ctx context.Context, country string) (*FormatSpec, error) {
	var out FormatSpec
	path := "/formats/" + url.PathEscape(strings.ToLower(country))
	err := c.request(ctx, http.MethodGet, path, nil, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// LookupBIC resolves an 8 or 11 character ISO 9362 BIC to a bank record.
func (c *Client) LookupBIC(ctx context.Context, bic string) (*BankRecord, error) {
	var out BankRecord
	path := "/swift/" + url.PathEscape(strings.ToUpper(bic))
	err := c.request(ctx, http.MethodGet, path, nil, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) request(ctx context.Context, method, path string, payload any, out any) error {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return &Error{Message: "could not encode the request body: " + err.Error(), kind: ErrAPI}
		}
		body = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return &Error{Message: "could not build the request: " + err.Error(), kind: ErrTransport}
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "ibanchecker-go/"+Version)
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return &Error{Message: fmt.Sprintf("request to %s failed: %s", c.baseURL+path, err), kind: ErrTransport}
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return &Error{Status: resp.StatusCode, Message: "could not read the response body: " + err.Error(), kind: ErrTransport}
	}

	// Decoded separately from out, so an error status still carries whatever
	// the API said even when it does not match the expected shape.
	var decoded map[string]any
	if len(bytes.TrimSpace(raw)) > 0 {
		if jsonErr := json.Unmarshal(raw, &decoded); jsonErr != nil {
			decoded = nil
		}
	}

	if resp.StatusCode >= 400 {
		return apiError(resp.StatusCode, decoded)
	}

	if decoded == nil {
		message := "the API returned a body that is not a JSON object"
		if len(bytes.TrimSpace(raw)) == 0 {
			message = "the API returned an empty body"
		}
		return &Error{Status: resp.StatusCode, Message: message, kind: ErrAPI}
	}

	if err := json.Unmarshal(raw, out); err != nil {
		return &Error{Status: resp.StatusCode, Message: "could not decode the response: " + err.Error(), Response: decoded, kind: ErrAPI}
	}
	return nil
}

func apiError(status int, decoded map[string]any) *Error {
	e := &Error{Status: status, Response: decoded, Message: fmt.Sprintf("HTTP %d", status)}

	if message, ok := decoded["error"].(string); ok && message != "" {
		e.Message = message
	}
	if code, ok := decoded["error_code"].(string); ok {
		e.Code = code
	}

	switch status {
	case http.StatusBadRequest:
		e.kind = ErrBadRequest
	case http.StatusUnauthorized:
		e.kind = ErrAuthentication
	case http.StatusNotFound:
		e.kind = ErrNotFound
	case http.StatusTooManyRequests:
		e.kind = ErrRateLimit
	default:
		e.kind = ErrAPI
	}
	return e
}
