# Changelog

## 0.1.2

Documentation only.

- `LookupBIC` now needs an API key: the API answers HTTP 401 without one,
  returned as `ErrAuthentication`. `CountryFormat` is the only method that
  still works without a key, and the limit of 100 requests an hour per IP now
  applies to it alone
- What a key can call follows its plan: a free key covers `Validate` only;
  `ValidateBulk` and `LookupBIC` need Basic or above (Basic, Starter, Growth,
  Enterprise); `Extract` needs Growth or above (Growth, Enterprise)
- A key whose email address has a verified account at
  ibanchecker.cash/dashboard can try the methods its plan lacks:
  `ValidateBulk` up to 10 IBANs per call, `LookupBIC`, and `Extract` up to
  5,000 characters per call. Over the trial size the API answers HTTP 400 with
  `TOO_MANY_IBANS` or `TEXT_TOO_LONG`, returned as `ErrBadRequest`
- A call outside the key's plan gets HTTP 403 with the code `PLAN_REQUIRED`;
  the client returns it as an `*Error` that unwraps to `ErrAPI`, with
  `required_plan` and `upgrade_url` in `Response`
- `ValidateBulk` counts one request per IBAN and `Extract` one per IBAN found
  (at least one per call); a call that costs more than the requests left this
  month gets HTTP 429 `QUOTA_EXCEEDED`, returned as `ErrRateLimit`
- README and godoc updated to match; the version goes to 0.1.2

The client's behaviour does not change, apart from the version in its
User-Agent header.

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
