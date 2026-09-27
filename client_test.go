package ibanchecker

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// recorded is what the fake server saw.
type recorded struct {
	method string
	path   string
	body   string
	header http.Header
}

// newServer replays status and body, and records every request.
func newServer(t *testing.T, status int, body any) (*Client, *[]recorded) {
	t.Helper()

	var payload string
	switch b := body.(type) {
	case string:
		payload = b
	default:
		encoded, err := json.Marshal(b)
		if err != nil {
			t.Fatalf("encoding the canned body: %v", err)
		}
		payload = string(encoded)
	}

	calls := make([]recorded, 0, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		read, _ := io.ReadAll(r.Body)
		calls = append(calls, recorded{method: r.Method, path: r.URL.EscapedPath(), body: string(read), header: r.Header.Clone()})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, payload)
	}))
	t.Cleanup(srv.Close)

	return New("", WithBaseURL(srv.URL)), &calls
}

func last(t *testing.T, calls *[]recorded) recorded {
	t.Helper()
	if len(*calls) == 0 {
		t.Fatal("the client made no request")
	}
	return (*calls)[len(*calls)-1]
}

func TestValidateReturnsAPopulatedResult(t *testing.T) {
	client, calls := newServer(t, 200, map[string]any{
		"valid":                true,
		"iban":                 "DE89370400440532013000",
		"formatted":            "DE89 3704 0044 0532 0130 00",
		"country":              "DE",
		"country_name":         "Germany",
		"bank_name":            "Commerzbank AG Cologne",
		"bic":                  "COBADEFFXXX",
		"bank_city":            "Köln",
		"sepa":                 true,
		"national_check_valid": true,
	})

	result, err := client.Validate(context.Background(), "DE89 3704 0044 0532 0130 00")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Valid {
		t.Error("Valid should be true")
	}
	if result.CountryName != "Germany" {
		t.Errorf("CountryName = %q", result.CountryName)
	}
	if result.BIC != "COBADEFFXXX" {
		t.Errorf("BIC = %q", result.BIC)
	}
	if result.BankCity != "Köln" {
		t.Errorf("BankCity = %q", result.BankCity)
	}
	if result.SEPA == nil || !*result.SEPA {
		t.Errorf("SEPA = %v, want a pointer to true", result.SEPA)
	}
	if result.NationalCheckValid == nil || !*result.NationalCheckValid {
		t.Errorf("NationalCheckValid = %v, want a pointer to true", result.NationalCheckValid)
	}
	if result.Raw["iban"] != "DE89370400440532013000" {
		t.Errorf("Raw did not keep the body: %v", result.Raw)
	}

	call := last(t, calls)
	if call.method != http.MethodPost {
		t.Errorf("method = %s", call.method)
	}
	if call.path != "/validate" {
		t.Errorf("path = %s", call.path)
	}
	if call.body != `{"iban":"DE89 3704 0044 0532 0130 00"}` {
		t.Errorf("body = %s", call.body)
	}
	if got := call.header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q", got)
	}
}

func TestAMalformedIBANIsAResultNotAnError(t *testing.T) {
	client, _ := newServer(t, 200, map[string]any{
		"valid":      false,
		"iban":       "XX00",
		"country":    "XX",
		"error":      `"XX" is not a recognized IBAN country code.`,
		"error_code": "INVALID_COUNTRY",
	})

	result, err := client.Validate(context.Background(), "XX00")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Error("Valid should be false")
	}
	if result.ErrorCode != "INVALID_COUNTRY" {
		t.Errorf("ErrorCode = %q", result.ErrorCode)
	}
	if result.NationalCheckValid != nil {
		t.Errorf("NationalCheckValid should stay nil when the API omits it, got %v", *result.NationalCheckValid)
	}
	if result.BankName != "" {
		t.Errorf("BankName = %q", result.BankName)
	}
}

func TestANationalCheckOfFalseIsNotAbsent(t *testing.T) {
	client, _ := newServer(t, 200, map[string]any{
		"valid": true, "iban": "DE84100100100532013000", "national_check_valid": false,
	})

	result, err := client.Validate(context.Background(), "DE84100100100532013000")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NationalCheckValid == nil {
		t.Fatal("NationalCheckValid should be a pointer to false, not nil")
	}
	if *result.NationalCheckValid {
		t.Error("NationalCheckValid should be false")
	}
}

func TestAPIKeyBecomesABearerHeaderAndIsOmittedWithoutOne(t *testing.T) {
	withKey, calls := newServer(t, 200, map[string]any{"valid": true, "iban": "DE89"})
	withKey.apiKey = "iban_test_key"
	if _, err := withKey.Validate(context.Background(), "DE89"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := last(t, calls).header.Get("Authorization"); got != "Bearer iban_test_key" {
		t.Errorf("Authorization = %q", got)
	}

	withoutKey, calls2 := newServer(t, 200, map[string]any{"valid": true, "iban": "DE89"})
	if _, err := withoutKey.Validate(context.Background(), "DE89"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, present := last(t, calls2).header["Authorization"]; present {
		t.Error("Authorization should be absent without an API key")
	}
}

func TestTheUserAgentCarriesTheClientVersion(t *testing.T) {
	client, calls := newServer(t, 200, map[string]any{"valid": true, "iban": "DE89"})
	if _, err := client.Validate(context.Background(), "DE89"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := last(t, calls).header.Get("User-Agent"), "ibanchecker-go/"+Version; got != want {
		t.Errorf("User-Agent = %q, want %q", got, want)
	}
}

func TestValidateBulkKeepsInputOrderAndCounts(t *testing.T) {
	client, calls := newServer(t, 200, map[string]any{
		"count": 2, "valid_count": 1, "invalid_count": 1,
		"results": []any{
			map[string]any{"valid": true, "iban": "DE89370400440532013000"},
			map[string]any{"valid": false, "iban": "XX00"},
		},
	})

	batch, err := client.ValidateBulk(context.Background(), []string{"DE89370400440532013000", "XX00"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if batch.Count != 2 || batch.ValidCount != 1 || batch.InvalidCount != 1 {
		t.Errorf("counts = %d/%d/%d", batch.Count, batch.ValidCount, batch.InvalidCount)
	}
	if len(batch.Results) != 2 {
		t.Fatalf("len(Results) = %d", len(batch.Results))
	}
	if batch.Results[0].IBAN != "DE89370400440532013000" || batch.Results[1].IBAN != "XX00" {
		t.Errorf("input order not preserved: %q, %q", batch.Results[0].IBAN, batch.Results[1].IBAN)
	}
	if batch.Results[0].Raw["iban"] != "DE89370400440532013000" {
		t.Error("nested results should keep their own Raw")
	}
	if body := last(t, calls).body; body != `{"ibans":["DE89370400440532013000","XX00"]}` {
		t.Errorf("body = %s", body)
	}
}

func TestValidateBulkSendsAnEmptyArrayNotNull(t *testing.T) {
	client, calls := newServer(t, 200, map[string]any{"count": 0, "results": []any{}})
	if _, err := client.ValidateBulk(context.Background(), nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if body := last(t, calls).body; body != `{"ibans":[]}` {
		t.Errorf("body = %s, want an empty array", body)
	}
}

func TestExtractReadsIBANsOutOfText(t *testing.T) {
	client, calls := newServer(t, 200, map[string]any{
		"count": 1, "valid_count": 1, "invalid_count": 0,
		"results": []any{map[string]any{
			"valid": true, "iban": "DE89370400440532013000", "bank_name": "Commerzbank AG Cologne",
		}},
	})

	batch, err := client.Extract(context.Background(), "Please wire to DE89 3704 0044 0532 0130 00 by Friday.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(batch.Results) != 1 || batch.Results[0].BankName != "Commerzbank AG Cologne" {
		t.Errorf("results = %+v", batch.Results)
	}
	if path := last(t, calls).path; path != "/extract" {
		t.Errorf("path = %s", path)
	}
}

func TestCountryFormatParsesBbanFields(t *testing.T) {
	client, calls := newServer(t, 200, map[string]any{
		"country_code": "DE", "country_name": "Germany", "length": 22,
		"sepa": true, "swift": true, "example": "DE89370400440532013000",
		"bban_fields": []any{
			map[string]any{"label": "BLZ", "length": 8, "type": "numeric", "description": "8-digit Bankleitzahl"},
			map[string]any{"label": "Account No.", "length": 10, "type": "numeric"},
		},
	})

	spec, err := client.CountryFormat(context.Background(), "DE")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec.Length != 22 {
		t.Errorf("Length = %d", spec.Length)
	}
	if spec.SEPA == nil || !*spec.SEPA {
		t.Error("SEPA should be a pointer to true")
	}
	if len(spec.BbanFields) != 2 {
		t.Fatalf("len(BbanFields) = %d", len(spec.BbanFields))
	}
	if spec.BbanFields[0].Label != "BLZ" || spec.BbanFields[0].Length != 8 {
		t.Errorf("first field = %+v", spec.BbanFields[0])
	}
	if spec.BbanFields[1].Description != "" {
		t.Errorf("Description = %q, want empty", spec.BbanFields[1].Description)
	}

	call := last(t, calls)
	if call.method != http.MethodGet {
		t.Errorf("method = %s", call.method)
	}
	if call.path != "/formats/de" {
		t.Errorf("path = %s, want the country code lowercased", call.path)
	}
	if call.body != "" {
		t.Errorf("GET should send no body, got %q", call.body)
	}
}

func TestLookupBICUppercasesThePath(t *testing.T) {
	client, calls := newServer(t, 200, map[string]any{
		"bic": "DEUTDEFFXXX", "bic8": "DEUTDEFF",
		"bank_name": "Deutsche Bank AG Frankfurt", "city": "FRANKFURT AM MAIN",
		"sepa": true, "status": "active",
	})

	bank, err := client.LookupBIC(context.Background(), "deutdeff")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bank.BankName != "Deutsche Bank AG Frankfurt" || bank.BIC8 != "DEUTDEFF" || bank.Status != "active" {
		t.Errorf("bank = %+v", bank)
	}
	if path := last(t, calls).path; path != "/swift/DEUTDEFF" {
		t.Errorf("path = %s", path)
	}
}

func TestPathSegmentsAreEscaped(t *testing.T) {
	client, calls := newServer(t, 200, map[string]any{"bic": "X"})
	if _, err := client.LookupBIC(context.Background(), "de/../admin"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path := last(t, calls).path; path != "/swift/DE%2F..%2FADMIN" {
		t.Errorf("path = %s, want the separators escaped", path)
	}
}

func TestErrorStatusesMapToSentinels(t *testing.T) {
	cases := map[int]error{
		400: ErrBadRequest,
		401: ErrAuthentication,
		403: ErrAPI,
		404: ErrNotFound,
		429: ErrRateLimit,
		500: ErrAPI,
		503: ErrAPI,
	}

	for status, want := range cases {
		client, _ := newServer(t, status, map[string]any{"error": "nope", "error_code": "SOME_CODE"})

		_, err := client.LookupBIC(context.Background(), "ZZZZZZZZ")
		if err == nil {
			t.Fatalf("HTTP %d: expected an error", status)
		}
		if !errors.Is(err, want) {
			t.Errorf("HTTP %d: errors.Is did not match the sentinel, got %v", status, err)
		}

		var apiErr *Error
		if !errors.As(err, &apiErr) {
			t.Fatalf("HTTP %d: errors.As should yield *Error", status)
		}
		if apiErr.Status != status {
			t.Errorf("HTTP %d: Status = %d", status, apiErr.Status)
		}
		if apiErr.Code != "SOME_CODE" {
			t.Errorf("HTTP %d: Code = %q", status, apiErr.Code)
		}
		if apiErr.Message != "nope" {
			t.Errorf("HTTP %d: Message = %q", status, apiErr.Message)
		}
		if apiErr.Response["error_code"] != "SOME_CODE" {
			t.Errorf("HTTP %d: Response not kept: %v", status, apiErr.Response)
		}
		if !strings.Contains(apiErr.Error(), "nope") {
			t.Errorf("HTTP %d: Error() = %q", status, apiErr.Error())
		}
	}
}

func TestAnErrorStatusWithoutAJSONBodyStillCarriesTheStatus(t *testing.T) {
	client, _ := newServer(t, 502, "<html>bad gateway</html>")

	_, err := client.Validate(context.Background(), "DE89")
	if !errors.Is(err, ErrAPI) {
		t.Fatalf("want ErrAPI, got %v", err)
	}

	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatal("want *Error")
	}
	if apiErr.Status != 502 || apiErr.Message != "HTTP 502" || apiErr.Code != "" || apiErr.Response != nil {
		t.Errorf("apiErr = %+v", apiErr)
	}
}

func TestANonJSONBodyOnASuccessfulStatusIsAnError(t *testing.T) {
	client, _ := newServer(t, 200, "<html>maintenance</html>")

	_, err := client.Validate(context.Background(), "DE89")
	if !errors.Is(err, ErrAPI) {
		t.Fatalf("want ErrAPI, got %v", err)
	}
	if !strings.Contains(err.Error(), "not a JSON object") {
		t.Errorf("Error() = %q", err.Error())
	}
}

func TestAnEmptyBodyOnASuccessfulStatusIsAnError(t *testing.T) {
	client, _ := newServer(t, 200, "")

	_, err := client.Validate(context.Background(), "DE89")
	if !errors.Is(err, ErrAPI) {
		t.Fatalf("want ErrAPI, got %v", err)
	}
	if !strings.Contains(err.Error(), "empty body") {
		t.Errorf("Error() = %q", err.Error())
	}
}

func TestBaseURLOverrideDropsATrailingSlash(t *testing.T) {
	client := New("", WithBaseURL("https://example.test/v1/"))
	if client.baseURL != "https://example.test/v1" {
		t.Errorf("baseURL = %q", client.baseURL)
	}
}

func TestAFailedRequestIsATransportError(t *testing.T) {
	// Reserved for documentation by RFC 2606, so it resolves nowhere.
	client := New("", WithBaseURL("https://ibanchecker.invalid/api/v1"))

	_, err := client.CountryFormat(context.Background(), "de")
	if !errors.Is(err, ErrTransport) {
		t.Fatalf("want ErrTransport, got %v", err)
	}
}

func TestARedirectIsRefusedWithTheTargetNamed(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// What the real endpoint answers a GET, which is what net/http would
		// downgrade this POST to on a 301.
		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	t.Cleanup(target.Close)

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+r.URL.Path, http.StatusMovedPermanently)
	}))
	t.Cleanup(redirector.Close)

	client := New("", WithBaseURL(redirector.URL))

	_, err := client.Validate(context.Background(), "DE89370400440532013000")
	if !errors.Is(err, ErrTransport) {
		t.Fatalf("want ErrTransport, got %v", err)
	}
	if !strings.Contains(err.Error(), "refusing to follow the redirect") {
		t.Errorf("Error() = %q, want the refusal stated", err.Error())
	}
	if !strings.Contains(err.Error(), target.URL) {
		t.Errorf("Error() = %q, want the redirect target named", err.Error())
	}
}

func TestACancelledContextStopsTheRequest(t *testing.T) {
	client, _ := newServer(t, 200, map[string]any{"valid": true, "iban": "DE89"})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.Validate(ctx, "DE89")
	if !errors.Is(err, ErrTransport) {
		t.Fatalf("want ErrTransport, got %v", err)
	}
	if !errors.Is(err, context.Canceled) {
		// The transport error wraps only the sentinel, so the cause is in the
		// message rather than the chain. Assert on that instead.
		if !strings.Contains(err.Error(), "context canceled") {
			t.Errorf("Error() = %q, want the cancellation named", err.Error())
		}
	}
}
