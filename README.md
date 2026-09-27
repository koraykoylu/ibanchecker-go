# ibanchecker-go

Official Go client for the [ibanchecker.cash](https://ibanchecker.cash) IBAN validation API.

Validate IBANs across 92 countries, validate up to 100 IBANs per request, extract IBANs from free text, look up country format specifications, and resolve SWIFT/BIC codes. No IBAN data is stored or logged; all validation runs in memory at the edge.

[![Go Reference](https://pkg.go.dev/badge/github.com/koraykoylu/ibanchecker-go.svg)](https://pkg.go.dev/github.com/koraykoylu/ibanchecker-go)

## Install

```bash
go get github.com/koraykoylu/ibanchecker-go
```

Requires Go 1.21 or newer. There are no dependencies: the client is built on `net/http` and `encoding/json` from the standard library.

## Quick start

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	ibanchecker "github.com/koraykoylu/ibanchecker-go"
)

func main() {
	// Every method except CountryFormat needs an API key.
	client := ibanchecker.New(os.Getenv("IBANCHECKER_API_KEY"))

	result, err := client.Validate(context.Background(), "DE89 3704 0044 0532 0130 00")
	if err != nil {
		log.Fatal(err)
	}

	if result.Valid {
		fmt.Println(result.CountryName) // Germany
		fmt.Println(result.BankName)    // Commerzbank AG Cologne
		fmt.Println(result.BIC)         // COBADEFFXXX
	} else {
		fmt.Println(result.Error)     // human-readable reason
		fmt.Println(result.ErrorCode) // e.g. INVALID_COUNTRY
	}
}
```

Every method takes a `context.Context` as its first argument, so a caller's deadline and cancellation reach the request.

## Authentication

Every method except `CountryFormat` needs an API key, `LookupBIC` included. Without one the API answers HTTP 401 and the client returns `ErrAuthentication`. A free key covers 100 requests a month and arrives by email in seconds: request it at [ibanchecker.cash/api-docs](https://ibanchecker.cash/api-docs). Paid plans are at [ibanchecker.cash/pricing](https://ibanchecker.cash/pricing).

What a key can call follows its plan:

- A free key covers `Validate` only.
- `ValidateBulk` and `LookupBIC` need the Basic plan or above (Basic, Starter, Growth, Enterprise).
- `Extract` needs the Growth plan or above (Growth, Enterprise).

A key whose email address has a verified account at [ibanchecker.cash/dashboard](https://ibanchecker.cash/dashboard) can try the methods its plan lacks: `ValidateBulk` with up to 10 IBANs per call, `LookupBIC`, and `Extract` with up to 5,000 characters per call. The trial applies to any plan that lacks the method, so a Basic key with a verified account can try `Extract`. A trial call over that size gets HTTP 400 with the code `TOO_MANY_IBANS` (bulk) or `TEXT_TOO_LONG` (extraction), returned as `ErrBadRequest`.

`CountryFormat` works without a key, limited to 100 requests an hour per IP; beyond that the API answers HTTP 429 with the code `RATE_LIMIT_EXCEEDED`. The hourly limit applies to country formats only.

```go
client := ibanchecker.New("YOUR_API_KEY")
client := ibanchecker.New(os.Getenv("IBANCHECKER_API_KEY"))

formats := ibanchecker.New("") // CountryFormat only
```

The key is sent as `Authorization: Bearer <key>`. Requests made with it count against its monthly quota: `Validate` and `LookupBIC` count one request per call, `ValidateBulk` one per IBAN in the call, and `Extract` one per IBAN found (at least one per call). Once the quota is used up the API answers HTTP 429 with the code `QUOTA_EXCEEDED` until the 1st of the next month (UTC), and a call that costs more than the requests left this month gets the same answer.

### A method outside the key's plan

Outside the trial, a call the key's plan does not include gets HTTP 403 with the code `PLAN_REQUIRED`. The client has no sentinel of its own for 403, so the error unwraps to `ErrAPI`; tell it apart by `Status` or `Code`, and read `required_plan` (`"basic"` or `"growth"`) and `upgrade_url` from `Response`:

```go
var apiErr *ibanchecker.Error
if errors.As(err, &apiErr) && apiErr.Code == "PLAN_REQUIRED" {
	fmt.Println("Needs the", apiErr.Response["required_plan"], "plan:", apiErr.Response["upgrade_url"])
}
```

## Methods

| Method | API key | Description |
| --- | --- | --- |
| `Validate(ctx, iban)` | required, any plan | Validate a single IBAN. Returns a `*ValidationResult`. |
| `ValidateBulk(ctx, ibans)` | required, Basic or above | Validate up to 100 IBANs (10 on the trial). Returns a `*BatchResult`. |
| `Extract(ctx, text)` | required, Growth or above | Find and validate IBANs in free text (up to 50,000 chars; 5,000 on the trial). Returns a `*BatchResult`. |
| `CountryFormat(ctx, country)` | optional | IBAN format spec for an ISO country code. Returns a `*FormatSpec`. |
| `LookupBIC(ctx, bic)` | required, Basic or above | Resolve an 8 or 11 character BIC. Returns a `*BankRecord`. |

### Bulk validation

```go
batch, err := client.ValidateBulk(ctx, []string{
	"DE89370400440532013000",
	"GB29NWBK60161331926819",
	"XX00",
})
if err != nil {
	log.Fatal(err)
}

fmt.Println(batch.ValidCount, "of", batch.Count, "valid")

for _, r := range batch.Results { // results come back in input order
	fmt.Println(r.IBAN, r.Valid)
}
```

### Extract from text

```go
batch, err := client.Extract(ctx, "Please wire to DE89 3704 0044 0532 0130 00 by Friday.")
for _, r := range batch.Results {
	fmt.Println(r.IBAN, r.BankName)
}
```

### Country format and BIC lookup

`CountryFormat` works without a key, limited to 100 requests an hour per IP. `LookupBIC` needs a key on the Basic plan or above, or a key with a verified account on the trial.

```go
spec, err := client.CountryFormat(ctx, "DE")
fmt.Println(spec.Length, spec.Example) // 22 DE89370400440532013000

for _, f := range spec.BbanFields {
	fmt.Printf("%s (%d)\n", f.Label, f.Length) // BLZ (8), Account No. (10)
}

bank, err := client.LookupBIC(ctx, "DEUTDEFF")
fmt.Println(bank.BankName, bank.City) // Deutsche Bank AG Frankfurt  FRANKFURT AM MAIN
```

### Tri-state fields are pointers

`NationalCheckValid`, `SEPA` and `SWIFT` are `*bool`, not `bool`. The API distinguishes false from absent and Go's zero value cannot, so a nil pointer means "not known for this IBAN" rather than "no".

```go
result, err := client.Validate(ctx, "DE84100100100532013000")
if result.Valid && result.NationalCheckValid != nil && !*result.NationalCheckValid {
	fmt.Println("Valid IBAN, but the account number looks mistyped.")
}
```

`NationalCheckValid` reports a domestic account check digit run on top of the ISO 13616 checksum, such as Germany's per-bank Prüfziffer or the UK sort-code and account modulus check. It is advisory: an IBAN with `Valid` true is a valid IBAN whatever this says.

## Error handling

A malformed IBAN is **not** an error: `Validate` returns a `*ValidationResult` with `Valid` false. Errors come back for transport, authentication, plan, quota and server-side problems only.

Compare with `errors.Is` for the common branches, and reach for `errors.As` when you need the detail:

```go
bank, err := client.LookupBIC(ctx, "ZZZZZZZZ")
switch {
case errors.Is(err, ibanchecker.ErrNotFound):
	fmt.Println("No bank for that BIC")
case errors.Is(err, ibanchecker.ErrRateLimit):
	fmt.Println("The monthly quota is used up, or this call costs more than is left")
case errors.Is(err, ibanchecker.ErrAuthentication):
	fmt.Println("Missing or invalid API key")
}

var apiErr *ibanchecker.Error
if errors.As(err, &apiErr) {
	fmt.Println(apiErr.Status, apiErr.Code, apiErr.Message, apiErr.Response)
}
```

A `QUOTA_EXCEEDED` error also carries `upgrade_url` in `apiErr.Response`. A `PLAN_REQUIRED` error (HTTP 403) has no sentinel of its own and unwraps to `ErrAPI`; it carries `required_plan` and `upgrade_url` in `apiErr.Response`.

| Sentinel | Returned when |
| --- | --- |
| `ErrBadRequest` | HTTP 400, the request was malformed, or a trial call went over the trial size (`TOO_MANY_IBANS`, `TEXT_TOO_LONG`) |
| `ErrAuthentication` | HTTP 401, the API key is missing (every method except `CountryFormat` needs one, `LookupBIC` included), invalid or inactive |
| `ErrNotFound` | HTTP 404, no such country code or BIC |
| `ErrRateLimit` | HTTP 429, the key's monthly quota is used up or the call costs more than is left (`QUOTA_EXCEEDED`), or `CountryFormat` without a key went over 100 an hour (`RATE_LIMIT_EXCEEDED`) |
| `ErrAPI` | any other error status, including HTTP 403 `PLAN_REQUIRED` (the key's plan does not include the method), or a body that could not be read |
| `ErrTransport` | the request never reached the API: DNS, TLS, connection, timeout |

## Timeouts and your own HTTP stack

```go
client := ibanchecker.New(apiKey, ibanchecker.WithTimeout(3*time.Second))

client := ibanchecker.New(apiKey, ibanchecker.WithHTTPClient(myClient))
```

`WithHTTPClient` replaces the whole client, so the timeout and the redirect policy below come from yours.

### Redirects are refused, not followed

The default client stops at a redirect and returns an `ErrTransport` naming the target. That is deliberate, for two reasons. `net/http` turns a POST into a GET on a 301, as the RFC asks, so an `http://` base URL would reach the https endpoint as a GET and come back `405 Method Not Allowed` with nothing to explain it. And a redirect to another host is how an API key travels somewhere it was never meant to go.

Use an `https` base URL and the situation does not arise.

## Raw responses

Every model keeps the untouched response body in `Raw`, so a field added to the API later is reachable without waiting for a client release.

```go
result, _ := client.Validate(ctx, "DE89370400440532013000")
fmt.Println(result.Raw["transfer_type"]) // SEPA+SWIFT
```

## Tests

```bash
go test ./...
```

The suite runs against `httptest` servers, so it needs no network.

## Links

- Website: https://ibanchecker.cash
- API documentation: https://ibanchecker.cash/api-docs
- OpenAPI spec: https://ibanchecker.cash/openapi.json
- Free online tools: https://ibanchecker.cash/tools

Clients for other languages: [Python](https://pypi.org/project/ibanchecker/), [PHP](https://packagist.org/packages/ibanchecker/client), [JavaScript](https://www.npmjs.com/package/@ibanchecker/client), [Ruby](https://rubygems.org/gems/ibanchecker), and an [MCP server](https://www.npmjs.com/package/@ibanchecker/mcp).

## License

MIT
