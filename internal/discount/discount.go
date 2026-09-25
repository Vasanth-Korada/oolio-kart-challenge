package discount

import (
	"errors"
	"fmt"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/coupon"
)

// CodeRule gives one coupon code its own percent. Codes are a list, not a
// JSON object keyed by code, because viper lowercases map keys and coupon
// codes are case-sensitive.
type CodeRule struct {
	Code    string  `mapstructure:"code"`
	Percent float64 `mapstructure:"percent"`
}

// Config is the "discount" section of the config file.
type Config struct {
	// DefaultPercent applies to every valid code without its own rule.
	DefaultPercent float64    `mapstructure:"defaultPercent"`
	Codes          []CodeRule `mapstructure:"codes"`
}

// Validate reports every invalid rule at once.
func (c Config) Validate() error {
	var problems []error
	if err := validatePercent(c.DefaultPercent); err != nil {
		problems = append(problems, fmt.Errorf("discount.defaultPercent: %w", err))
	}
	seen := make(map[string]bool, len(c.Codes))
	for _, codeRule := range c.Codes {
		name := fmt.Sprintf("discount.codes[%q]", codeRule.Code)
		if !coupon.ValidLength(codeRule.Code) {
			problems = append(problems, fmt.Errorf("%s: code must be 8 to 10 characters", name))
		}
		if seen[codeRule.Code] {
			problems = append(problems, fmt.Errorf("%s: listed more than once", name))
		}
		seen[codeRule.Code] = true
		if err := validatePercent(codeRule.Percent); err != nil {
			problems = append(problems, fmt.Errorf("%s: %w", name, err))
		}
	}
	return errors.Join(problems...)
}

func validatePercent(percent float64) error {
	if percent <= 0 || percent > 100 {
		return fmt.Errorf("percent must be above 0 and at most 100, got %g", percent)
	}
	return nil
}

// Policy prices valid coupons from a Config. It implements
// order.DiscountPolicy and is safe for concurrent use (read-only after New).
type Policy struct {
	defaultPercent float64
	percentByCode  map[string]float64
}

// New validates cfg and returns its Policy.
func New(cfg Config) (*Policy, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	percentByCode := make(map[string]float64, len(cfg.Codes))
	for _, codeRule := range cfg.Codes {
		percentByCode[codeRule.Code] = codeRule.Percent
	}
	return &Policy{defaultPercent: cfg.DefaultPercent, percentByCode: percentByCode}, nil
}

// Discount returns the code's percent of subtotal: its own percent if it has
// a rule, else the default. Percents are at most 100, so the result never
// exceeds the subtotal. The caller rounds it to cents.
func (p *Policy) Discount(code string, subtotal float64) float64 {
	percent, ok := p.percentByCode[code]
	if !ok {
		percent = p.defaultPercent
	}
	// percent/100 first, so 5 gives exactly the old 0.05 rate.
	return subtotal * (percent / 100)
}

// Rules returns how many per-code rules the policy holds, for logging.
func (p *Policy) Rules() int { return len(p.percentByCode) }
