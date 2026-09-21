// Package coupon validates promo codes against a pre-built index of the
// codes found in at least 2 of Oolio's 3 supplied coupon files. See
// index.go for how that index is built and queried; this file only
// defines the seam the rest of the application depends on.
package coupon

// Validator decides whether a promo code is valid. The order service
// depends on this interface, never on Index directly, so the validation
// rule (and its backing data structure) can change without touching
// callers.
type Validator interface {
	IsValid(code string) bool
}

const (
	minLength = 8
	maxLength = 10
)

// ValidLength reports whether code satisfies the length precondition
// (8-10 characters) that applies before any file-membership check.
// Exported so callers/tests can distinguish a length failure from a
// membership failure without depending on Index internals.
func ValidLength(code string) bool {
	n := len(code)
	return n >= minLength && n <= maxLength
}

// unavailableValidator rejects every code. It's used when the index
// file hasn't been built yet, so the server can still start (and every
// other endpoint keeps working) instead of crashing on missing data —
// only orders with a coupon code are affected, and they fail with a
// clear validation error rather than a panic.
type unavailableValidator struct{}

func (unavailableValidator) IsValid(string) bool { return false }

// NewUnavailableValidator returns a Validator that rejects every code.
func NewUnavailableValidator() Validator { return unavailableValidator{} }

var _ Validator = unavailableValidator{}
