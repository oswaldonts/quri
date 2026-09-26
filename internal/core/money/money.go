// Package money represents monetary amounts as integer minor units (e.g. cents)
// tied to a currency, avoiding floating point rounding errors.
package money

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var (
	ErrUnsupportedCurrency = errors.New("unsupported currency")
	ErrInvalidAmount       = errors.New("invalid amount")
	ErrCurrencyMismatch    = errors.New("currency mismatch")
)

// Currency is an ISO 4217 currency code.
type Currency string

const (
	USD Currency = "USD"
	PEN Currency = "PEN"
)

// minorUnits maps each supported currency to its number of decimal places.
var minorUnits = map[Currency]int{
	USD: 2,
	PEN: 2,
}

// maxIntegerDigits keeps parsed amounts far from the int64 limit.
const maxIntegerDigits = 13

// ParseCurrency validates an ISO 4217 code (case-insensitive) against the supported currencies.
func ParseCurrency(code string) (Currency, error) {
	c := Currency(strings.ToUpper(strings.TrimSpace(code)))
	if _, ok := minorUnits[c]; !ok {
		return "", fmt.Errorf("%w: %q", ErrUnsupportedCurrency, code)
	}
	return c, nil
}

// Money is an amount expressed in the minor unit of its currency.
// Fields are unexported so a Money can only be built through Parse or New and never changes.
type Money struct {
	amount   int64
	currency Currency
}

// New builds Money from an amount already expressed in minor units (e.g. 133000 for 1330.00 USD).
func New(minor int64, c Currency) (Money, error) {
	if _, ok := minorUnits[c]; !ok {
		return Money{}, fmt.Errorf("%w: %q", ErrUnsupportedCurrency, c)
	}
	return Money{amount: minor, currency: c}, nil
}

// Parse converts a decimal string such as "1330", "1330.5" or "-7.00" into Money.
// It rejects more decimal places than the currency allows instead of rounding.
func Parse(s string, c Currency) (Money, error) {
	decimals, ok := minorUnits[c]
	if !ok {
		return Money{}, fmt.Errorf("%w: %q", ErrUnsupportedCurrency, c)
	}

	digits := strings.TrimSpace(s)
	negative := strings.HasPrefix(digits, "-")
	digits = strings.TrimPrefix(digits, "-")

	intPart, fracPart, hasDot := strings.Cut(digits, ".")
	switch {
	case intPart == "", len(intPart) > maxIntegerDigits, !isDigits(intPart):
		return Money{}, fmt.Errorf("%w: %q", ErrInvalidAmount, s)
	case hasDot && (fracPart == "" || !isDigits(fracPart)):
		return Money{}, fmt.Errorf("%w: %q", ErrInvalidAmount, s)
	case len(fracPart) > decimals:
		return Money{}, fmt.Errorf("%w: %q has more than %d decimal places", ErrInvalidAmount, s, decimals)
	}

	fracPart += strings.Repeat("0", decimals-len(fracPart))
	minor, err := strconv.ParseInt(intPart+fracPart, 10, 64)
	if err != nil {
		return Money{}, fmt.Errorf("%w: %q", ErrInvalidAmount, s)
	}
	if negative {
		minor = -minor
	}
	return Money{amount: minor, currency: c}, nil
}

// Amount returns the value in minor units.
func (m Money) Amount() int64 { return m.amount }

// Currency returns the ISO 4217 code.
func (m Money) Currency() Currency { return m.currency }

// IsPositive reports whether the amount is greater than zero.
func (m Money) IsPositive() bool { return m.amount > 0 }

// Add returns the sum of two amounts in the same currency.
func (m Money) Add(other Money) (Money, error) {
	if m.currency != other.currency {
		return Money{}, fmt.Errorf("%w: %s and %s", ErrCurrencyMismatch, m.currency, other.currency)
	}
	return Money{amount: m.amount + other.amount, currency: m.currency}, nil
}

// String formats the amount with the currency's decimal places, e.g. "1330.00".
func (m Money) String() string {
	decimals := minorUnits[m.currency]
	sign := ""
	amount := m.amount
	if amount < 0 {
		sign = "-"
		amount = -amount
	}
	if decimals == 0 {
		return sign + strconv.FormatInt(amount, 10)
	}
	scale := pow10(decimals)
	return fmt.Sprintf("%s%d.%0*d", sign, amount/scale, decimals, amount%scale)
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func pow10(n int) int64 {
	p := int64(1)
	for range n {
		p *= 10
	}
	return p
}
