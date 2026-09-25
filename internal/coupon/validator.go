package coupon

// Validator reports whether a coupon code is valid. order.Service depends on
// this interface, not on Index, so tests and the fail-closed fallback can
// stand in.
type Validator interface {
	IsValid(code string) bool
}

const (
	minLength = 8
	maxLength = 10
)

// ValidLength reports whether code has an allowed length (8 to 10 bytes).
// Both the build and the lookup apply it.
func ValidLength(code string) bool {
	length := len(code)
	return length >= minLength && length <= maxLength
}

type unavailableValidator struct{}

// IsValid always returns false.
func (unavailableValidator) IsValid(string) bool { return false }

// NewUnavailableValidator returns a Validator that rejects every code. The
// server uses it when the index file is missing, so no discount is given by
// mistake.
func NewUnavailableValidator() Validator { return unavailableValidator{} }

var _ Validator = unavailableValidator{}
