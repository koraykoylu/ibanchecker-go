# Changelog

## 0.1.1

Documentation only; the client behaves as before, apart from the version in
its User-Agent header.

- `Validate`, `ValidateBulk` and `Extract` now need an API key: the API
  answers HTTP 401 without one, returned as `ErrAuthentication`
- The free key covers 100 requests a month; over the quota the API answers
  HTTP 429 with the code `QUOTA_EXCEEDED`, returned as `ErrRateLimit`
- `CountryFormat` and `LookupBIC` still work without a key, limited to 100
  requests an hour per IP
- README, godoc and the quick start construct the client with a key

## 0.1.0

First release.

- `Validate`, `ValidateBulk`, `Extract`, `CountryFormat` and `LookupBIC`, each
  taking a `context.Context`
- Typed models for every response, with the raw body kept in `Raw`
- Sentinel errors for 400, 401, 404, 429, other statuses and transport
  failures, comparable with `errors.Is`, plus `*Error` through `errors.As` for
  the status, code and decoded body; a malformed IBAN is a result with `Valid`
  false, never an error
- Tri-state API fields are `*bool`, so absent stays distinguishable from false
- No dependencies; `net/http` and `encoding/json` only, with `WithHTTPClient`
  to route calls through an application's own stack
