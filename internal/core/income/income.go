// Package income registers the money received in each budget period.
package income

import (
	"errors"
	"fmt"
	"time"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/money"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/period"
)

var (
	// ErrInvalidInput wraps every validation error, so callers can tell a bad request
	// apart from an internal failure with errors.Is(err, ErrInvalidInput).
	ErrInvalidInput = errors.New("invalid input")
	// ErrNotFound means no income of the period has the given ID.
	ErrNotFound = errors.New("income not found")
	// ErrManagedByExchange means the income was created by an exchange and can only
	// change through it.
	ErrManagedByExchange = errors.New("income is managed by its exchange")
)

// Kind classifies an income.
type Kind string

const (
	KindSalary Kind = "salary"
	KindExtra  Kind = "extra"
	// KindExchange is money obtained by converting another currency.
	// It is created by the exchange service, never registered by hand.
	KindExchange Kind = "exchange"
)

// parseKind accepts only the kinds a user may register directly.
func parseKind(s string) (Kind, error) {
	switch k := Kind(s); k {
	case KindSalary, KindExtra:
		return k, nil
	case KindExchange:
		return "", fmt.Errorf("kind %q is created automatically when registering an exchange", s)
	default:
		return "", fmt.Errorf("kind %q (want %q or %q)", s, KindSalary, KindExtra)
	}
}

// Income is money received for a budget period in a single currency.
type Income struct {
	ID     string
	Period period.Period
	Amount money.Money
	Kind   Kind
	// CategoryID is optional and only for KindExtra: the category the extra money
	// is contributed to (an "aporte"), e.g. an exchange gain assigned to Créditos.
	CategoryID  string
	Description string
	ReceivedOn  time.Time
	CreatedAt   time.Time
}
