// Package coupon validates promo codes against a pre-built index of the
// codes found in at least 2 of Oolio's 3 supplied coupon files.
package coupon

// Validator decides whether a promo code is valid.
type Validator interface {
	IsValid(code string) bool
}

const (
	minLength = 8
	maxLength = 10
)

// ValidLength reports whether code satisfies the length precondition
// that applies before any file-membership check.
func ValidLength(code string) bool {
	n := len(code)
	return n >= minLength && n <= maxLength
}

// unavailableValidator rejects every code — used when the index file
// hasn't been built yet, so the server can still start instead of
// crashing on missing data.
type unavailableValidator struct{}

func (unavailableValidator) IsValid(string) bool { return false }

// NewUnavailableValidator returns a Validator that rejects every code.
func NewUnavailableValidator() Validator { return unavailableValidator{} }

var _ Validator = unavailableValidator{}
