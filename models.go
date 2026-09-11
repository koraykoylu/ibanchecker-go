package ibanchecker

import "encoding/json"

// Tri-state fields are pointers on purpose. The API distinguishes false from
// absent, and Go's zero value cannot: a nil *bool means "not known for this
// IBAN", which is not the same answer as false.

// ValidationResult is the result of validating one IBAN.
//
// Valid is the primary flag. When it is false only IBAN, Formatted, Country,
// CountryName, Error and ErrorCode are populated.
type ValidationResult struct {
	Valid       bool   `json:"valid"`
	IBAN        string `json:"iban"`
	Formatted   string `json:"formatted"`
	CheckDigits string `json:"check_digits"`
	BBAN        string `json:"bban"`
	Country     string `json:"country"`
	CountryName string `json:"country_name"`

	BankName      string `json:"bank_name"`
	BankType      string `json:"bank_type"`
	BIC           string `json:"bic"`
	BankCity      string `json:"bank_city"`
	BankCode      string `json:"bank_code"`
	BranchCode    string `json:"branch_code"`
	AccountNumber string `json:"account_number"`

	// NationalCheckValid reports a domestic account check digit run on top of
	// the ISO 13616 checksum, such as Germany's per-bank Pruefziffer or the UK
	// sort-code and account modulus check. It is advisory: an IBAN with Valid
	// true is a valid IBAN whatever this says. False usually means a
	// transcription error in the account number. Nil where the country has no
	// such scheme.
	NationalCheckValid *bool `json:"national_check_valid"`

	Currency     string `json:"currency"`
	CurrencyName string `json:"currency_name"`
	TransferType string `json:"transfer_type"`

	// SEPA reports whether the IBAN's country is in the SEPA zone. Nil when
	// the API did not say.
	SEPA *bool  `json:"sepa"`
	Flag string `json:"flag"`

	// Error and ErrorCode explain why Valid is false.
	Error     string `json:"error"`
	ErrorCode string `json:"error_code"`

	// Raw is the untouched response body, so a field added to the API later is
	// reachable without waiting for a client release.
	Raw map[string]any `json:"-"`
}

func (r *ValidationResult) UnmarshalJSON(data []byte) error {
	type alias ValidationResult
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*r = ValidationResult(a)
	return json.Unmarshal(data, &r.Raw)
}

// BatchResult is the result of a bulk validation or a text extraction. Results
// come back in the same order as the input.
type BatchResult struct {
	Count        int                `json:"count"`
	ValidCount   int                `json:"valid_count"`
	InvalidCount int                `json:"invalid_count"`
	Results      []ValidationResult `json:"results"`

	// Raw is the untouched response body.
	Raw map[string]any `json:"-"`
}

func (b *BatchResult) UnmarshalJSON(data []byte) error {
	type alias BatchResult
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*b = BatchResult(a)
	return json.Unmarshal(data, &b.Raw)
}

// BbanField is one segment of a country's BBAN, in the order it appears in the
// IBAN.
type BbanField struct {
	Label       string `json:"label"`
	Length      int    `json:"length"`
	Type        string `json:"type"`
	Description string `json:"description"`

	// Raw is the untouched field object.
	Raw map[string]any `json:"-"`
}

func (f *BbanField) UnmarshalJSON(data []byte) error {
	type alias BbanField
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*f = BbanField(a)
	return json.Unmarshal(data, &f.Raw)
}

// FormatSpec is the IBAN format specification for one country.
type FormatSpec struct {
	CountryCode  string `json:"country_code"`
	CountryName  string `json:"country_name"`
	Length       int    `json:"length"`
	Currency     string `json:"currency"`
	CurrencyName string `json:"currency_name"`

	SEPA  *bool `json:"sepa"`
	SWIFT *bool `json:"swift"`

	FormatString string      `json:"format_string"`
	Example      string      `json:"example"`
	BbanFields   []BbanField `json:"bban_fields"`

	// Raw is the untouched response body.
	Raw map[string]any `json:"-"`
}

func (s *FormatSpec) UnmarshalJSON(data []byte) error {
	type alias FormatSpec
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*s = FormatSpec(a)
	return json.Unmarshal(data, &s.Raw)
}

// BankRecord is the institution behind a SWIFT/BIC code.
type BankRecord struct {
	BIC          string `json:"bic"`
	BIC8         string `json:"bic8"`
	BankCode     string `json:"bank_code"`
	CountryCode  string `json:"country_code"`
	LocationCode string `json:"location_code"`
	BranchCode   string `json:"branch_code"`
	BankName     string `json:"bank_name"`
	City         string `json:"city"`
	CountryName  string `json:"country_name"`

	SEPA   *bool  `json:"sepa"`
	Type   string `json:"type"`
	Status string `json:"status"`

	// Raw is the untouched response body.
	Raw map[string]any `json:"-"`
}

func (b *BankRecord) UnmarshalJSON(data []byte) error {
	type alias BankRecord
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*b = BankRecord(a)
	return json.Unmarshal(data, &b.Raw)
}
