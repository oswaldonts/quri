// Package income registers the money received in each budget period.
package income

import (
	"errors"
	"fmt"
	"time"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/money"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/period"
)

// ErrInvalidInput wraps every validation error, so callers can tell a bad request
// apart from an internal failure with errors.Is(err, ErrInvalidInput).
var ErrInvalidInput = errors.New("invalid input")

// Kind classifies an income.
type Kind string

const (
	KindSalary Kind = "salary"
	KindExtra  Kind = "extra"
)

func parseKind(s string) (Kind, error) {
	switch k := Kind(s); k {
	case KindSalary, KindExtra:
		return k, nil
	default:
		return "", fmt.Errorf("kind %q (want %q or %q)", s, KindSalary, KindExtra)
	}
}

// Income is money received for a budget period in a single currency.
type Income struct {
	ID          string
	Period      period.Period
	Amount      money.Money
	Kind        Kind
	Description string
	ReceivedOn  time.Time
	CreatedAt   time.Time
}
