package coupon

type Validator interface {
	IsValid(code string) bool
}

const (
	minLength = 8
	maxLength = 10
)

func ValidLength(code string) bool {
	n := len(code)
	return n >= minLength && n <= maxLength
}

type unavailableValidator struct{}

func (unavailableValidator) IsValid(string) bool { return false }

func NewUnavailableValidator() Validator { return unavailableValidator{} }

var _ Validator = unavailableValidator{}
