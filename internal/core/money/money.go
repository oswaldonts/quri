// Package money represents monetary amounts as integer minor units (e.g. cents)
// tied to a currency, avoiding floating point rounding errors.
package money

import (
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

var (
	ErrUnsupportedCurrency = errors.New("unsupported currency")
	ErrInvalidAmount       = errors.New("invalid amount")
	ErrCurrencyMismatch    = errors.New("currency mismatch")
	ErrInvalidRate         = errors.New("invalid rate")
)

// Currency is a three-letter ISO 4217 style code. Which codes are in use is up to the user
// (see package currency); this package only checks the shape.
type Currency string

// Codes the tests and API examples use; they are not a list of supported currencies.
const (
	USD Currency = "USD"
	PEN Currency = "PEN"
	ARS Currency = "ARS"
	COP Currency = "COP"
	VES Currency = "VES"
)

// decimals is the number of minor-unit places of every currency.
const decimals = 2

func valid(c Currency) bool {
	if len(c) != 3 {
		return false
	}
	for _, r := range c {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

// maxAmountDigits keeps parsed amounts far from the int64 limit.
const maxAmountDigits = 13

// ParseCurrency validates the shape of a currency code (three letters, case-insensitive).
func ParseCurrency(code string) (Currency, error) {
	c := Currency(strings.ToUpper(strings.TrimSpace(code)))
	if !valid(c) {
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
	if !valid(c) {
		return Money{}, fmt.Errorf("%w: %q", ErrUnsupportedCurrency, c)
	}
	return Money{amount: minor, currency: c}, nil
}

// Parse converts a decimal string such as "1330", "1330.5" or "-7.00" into Money.
// It rejects more decimal places than the currency allows instead of rounding.
func Parse(s string, c Currency) (Money, error) {
	if !valid(c) {
		return Money{}, fmt.Errorf("%w: %q", ErrUnsupportedCurrency, c)
	}
	minor, err := parseDecimal(s, maxAmountDigits, decimals)
	if err != nil {
		return Money{}, fmt.Errorf("%w: %w", ErrInvalidAmount, err)
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

// Zero returns an amount of zero in currency c (c must be a valid code).
func Zero(c Currency) Money {
	return Money{currency: c}
}

// Sub returns m minus other; both must be in the same currency.
func (m Money) Sub(other Money) (Money, error) {
	if m.currency != other.currency {
		return Money{}, fmt.Errorf("%w: %s and %s", ErrCurrencyMismatch, m.currency, other.currency)
	}
	return Money{amount: m.amount - other.amount, currency: m.currency}, nil
}

// Convert multiplies m by r and expresses the result in currency to.
// Any fraction smaller than the target minor unit is truncated toward zero,
// e.g. 1096.37 USD × 3.2567 = 3570.548179 → 3570.54 PEN.
func (m Money) Convert(r Rate, to Currency) (Money, error) {
	if !valid(to) {
		return Money{}, fmt.Errorf("%w: %q", ErrUnsupportedCurrency, to)
	}
	if r.scaled <= 0 {
		return Money{}, fmt.Errorf("%w: must be greater than zero", ErrInvalidRate)
	}

	// result = amount × rate × 10^toDecimals / 10^(RateDecimals + fromDecimals)
	// big.Int avoids int64 overflow in the intermediate product.
	n := new(big.Int).Mul(big.NewInt(m.amount), big.NewInt(r.scaled))
	n.Mul(n, big.NewInt(pow10(decimals)))
	n.Quo(n, big.NewInt(pow10(RateDecimals+decimals))) // Quo truncates toward zero

	if !n.IsInt64() {
		return Money{}, fmt.Errorf("%w: converted amount out of range", ErrInvalidAmount)
	}
	return Money{amount: n.Int64(), currency: to}, nil
}

// String formats the amount with the currency's decimal places, e.g. "1330.00".
func (m Money) String() string {
	return formatDecimal(m.amount, decimals)
}

// RateDecimals is the precision of exchange rates. 8 places keep small rates exact
// enough, e.g. 1 COP = 0.00025 USD.
const RateDecimals = 8

// maxRateDigits bounds the integer part so 10 + 8 digits fit in an int64.
const maxRateDigits = 10

// Rate is an exchange rate stored as an integer scaled by 10^8 (3.25 → 325000000).
type Rate struct {
	scaled int64
}

// ParseRate reads a positive decimal rate with up to 8 decimal places, e.g. "3.25" or "0.00025".
func ParseRate(s string) (Rate, error) {
	scaled, err := parseDecimal(s, maxRateDigits, RateDecimals)
	if err != nil {
		return Rate{}, fmt.Errorf("%w: %w", ErrInvalidRate, err)
	}
	if scaled <= 0 {
		return Rate{}, fmt.Errorf("%w: %q must be greater than zero", ErrInvalidRate, s)
	}
	return Rate{scaled: scaled}, nil
}

// NewRate builds a Rate from its scaled integer form (e.g. 325000000 for 3.25), as stored in a database.
func NewRate(scaled int64) (Rate, error) {
	if scaled <= 0 {
		return Rate{}, fmt.Errorf("%w: %d must be greater than zero", ErrInvalidRate, scaled)
	}
	return Rate{scaled: scaled}, nil
}

// Scaled returns the rate multiplied by 10^8 (3.25 → 325000000), for storage.
func (r Rate) Scaled() int64 { return r.scaled }

// String formats the rate without trailing zeros, e.g. "3.25", "1150" or "0.00025".
func (r Rate) String() string {
	s := formatDecimal(r.scaled, RateDecimals)
	s = strings.TrimRight(s, "0")
	return strings.TrimSuffix(s, ".")
}

// parseDecimal converts a decimal string into an integer scaled by 10^decimals
// ("142.5" with 2 decimals → 14250). It never rounds: extra decimal places are an error.
// maxIntegerDigits bounds the integer part so the result fits in an int64.
func parseDecimal(s string, maxIntegerDigits, decimals int) (int64, error) {
	digits := strings.TrimSpace(s)
	negative := strings.HasPrefix(digits, "-")
	digits = strings.TrimPrefix(digits, "-")

	intPart, fracPart, hasDot := strings.Cut(digits, ".")
	switch {
	case intPart == "", len(intPart) > maxIntegerDigits, !isDigits(intPart):
		return 0, fmt.Errorf("%q is not a valid decimal number", s)
	case hasDot && (fracPart == "" || !isDigits(fracPart)):
		return 0, fmt.Errorf("%q is not a valid decimal number", s)
	case len(fracPart) > decimals:
		return 0, fmt.Errorf("%q has more than %d decimal places", s, decimals)
	}

	fracPart += strings.Repeat("0", decimals-len(fracPart))
	v, err := strconv.ParseInt(intPart+fracPart, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%q is out of range", s)
	}
	if negative {
		v = -v
	}
	return v, nil
}

// formatDecimal is the inverse of parseDecimal: 14250 with 2 decimals → "142.50".
func formatDecimal(v int64, decimals int) string {
	sign := ""
	if v < 0 {
		sign = "-"
		v = -v
	}
	if decimals == 0 {
		return sign + strconv.FormatInt(v, 10)
	}
	scale := pow10(decimals)
	return fmt.Sprintf("%s%d.%0*d", sign, v/scale, decimals, v%scale)
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
