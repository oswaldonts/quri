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

// maxIntegerDigits keeps parsed values far from the int64 limit.
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
	minor, err := parseDecimal(s, decimals)
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

// Convert multiplies m by r and expresses the result in currency to.
// Any fraction smaller than the target minor unit is truncated toward zero,
// e.g. 1096.37 USD × 3.2567 = 3570.548179 → 3570.54 PEN.
func (m Money) Convert(r Rate, to Currency) (Money, error) {
	toDecimals, ok := minorUnits[to]
	if !ok {
		return Money{}, fmt.Errorf("%w: %q", ErrUnsupportedCurrency, to)
	}
	if r.scaled <= 0 {
		return Money{}, fmt.Errorf("%w: must be greater than zero", ErrInvalidRate)
	}
	fromDecimals := minorUnits[m.currency]

	// result = amount × rate × 10^toDecimals / 10^(rateDecimals + fromDecimals)
	// big.Int avoids int64 overflow in the intermediate product.
	n := new(big.Int).Mul(big.NewInt(m.amount), big.NewInt(r.scaled))
	n.Mul(n, big.NewInt(pow10(toDecimals)))
	n.Quo(n, big.NewInt(pow10(rateDecimals+fromDecimals))) // Quo truncates toward zero

	if !n.IsInt64() {
		return Money{}, fmt.Errorf("%w: converted amount out of range", ErrInvalidAmount)
	}
	return Money{amount: n.Int64(), currency: to}, nil
}

// String formats the amount with the currency's decimal places, e.g. "1330.00".
func (m Money) String() string {
	return formatDecimal(m.amount, minorUnits[m.currency])
}

// rateDecimals is the precision of exchange rates, e.g. 3.2567.
const rateDecimals = 4

// Rate is an exchange rate stored as an integer scaled by 10^rateDecimals (3.25 → 32500).
type Rate struct {
	scaled int64
}

// ParseRate reads a positive decimal rate with up to 4 decimal places, e.g. "3.25".
func ParseRate(s string) (Rate, error) {
	scaled, err := parseDecimal(s, rateDecimals)
	if err != nil {
		return Rate{}, fmt.Errorf("%w: %w", ErrInvalidRate, err)
	}
	if scaled <= 0 {
		return Rate{}, fmt.Errorf("%w: %q must be greater than zero", ErrInvalidRate, s)
	}
	return Rate{scaled: scaled}, nil
}

// String formats the rate with 4 decimal places, e.g. "3.2500".
func (r Rate) String() string {
	return formatDecimal(r.scaled, rateDecimals)
}

// parseDecimal converts a decimal string into an integer scaled by 10^decimals
// ("142.5" with 2 decimals → 14250). It never rounds: extra decimal places are an error.
func parseDecimal(s string, decimals int) (int64, error) {
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
