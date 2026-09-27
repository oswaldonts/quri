// Package exchange manages the exchange rate chosen for each budget period and
// the currency exchanges made with it.
//
// An exchange is an expense in the source currency (system category "exchange");
// the converted amount (source amount × period rate, truncated) is registered as
// an income in the target currency.
package exchange

import (
	"errors"
	"time"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/money"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/period"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/status"
)

var (
	// ErrInvalidInput wraps every validation error.
	ErrInvalidInput = errors.New("invalid input")
	// ErrRateNotFound means no rate was set for the period and currency pair.
	ErrRateNotFound = errors.New("rate not found")
	// ErrNotFound means no exchange of the period has the given ID.
	ErrNotFound = errors.New("exchange not found")
)

// PeriodRate is the rate chosen for a currency pair in a budget period:
// 1 unit of From = Rate units of To.
type PeriodRate struct {
	Period    period.Period
	From      money.Currency
	To        money.Currency
	Rate      money.Rate
	UpdatedAt time.Time
}

// Exchange converts money from one currency to another using the period rate.
// The rate is copied into the exchange so later rate changes don't alter it.
type Exchange struct {
	ID          string
	Period      period.Period
	From        money.Money // spent in the source currency
	To          money.Money // received in the target currency
	Rate        money.Rate
	Description string
	ExchangedOn time.Time
	IncomeID    string        // income created in the target currency
	Status      status.Status // planned or confirmed; shared by its income and expense
	CreatedAt   time.Time
}
