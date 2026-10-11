// Package balance records the money that really exists at the end of a period,
// per currency (accounts plus cash). The summary compares it with the expected
// balance (incomes − expenses) to detect entries that were not registered.
package balance

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/currency"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/money"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/period"
)

var (
	// ErrInvalidInput wraps every validation error.
	ErrInvalidInput = errors.New("invalid input")
	// ErrNotFound means the currency has no balance in the period.
	ErrNotFound = errors.New("balance not found")
)

// Balance is the real money in one currency at the end of a period.
type Balance struct {
	Period    period.Period
	Amount    money.Money // may be negative (e.g. a credit card debt)
	UpdatedAt time.Time
}

// Repository stores one balance per period and currency.
type Repository interface {
	// Save creates or replaces the balance of its period and currency.
	Save(ctx context.Context, b Balance) error
	// ListByPeriod returns the balances of the period ordered by currency.
	ListByPeriod(ctx context.Context, p period.Period) ([]Balance, error)
	// Delete removes the balance; it returns ErrNotFound if there is none.
	Delete(ctx context.Context, p period.Period, c money.Currency) error
}

// Service holds the balance use cases.
type Service struct {
	repo       Repository
	currencies currency.Checker
	now        func() time.Time
}

// NewService returns a Service backed by repo.
func NewService(repo Repository, currencies currency.Checker) *Service {
	return &Service{repo: repo, currencies: currencies, now: time.Now}
}

// SetInput carries the real balance of a currency in a period.
type SetInput struct {
	Period   string // YYYY-MM
	Currency string // ISO 4217 code
	Amount   string // decimal string, e.g. "415.50" or "-20.00"
}

// Set creates or replaces the real balance of a currency in a period.
func (s *Service) Set(ctx context.Context, in SetInput) (Balance, error) {
	p, err := period.Parse(in.Period)
	if err != nil {
		return Balance{}, invalid(err)
	}
	currency, err := money.ParseCurrency(in.Currency)
	if err != nil {
		return Balance{}, invalid(err)
	}
	if err := s.currencies.Require(ctx, currency); err != nil {
		return Balance{}, err
	}
	amount, err := money.Parse(in.Amount, currency)
	if err != nil {
		return Balance{}, invalid(err)
	}

	b := Balance{Period: p, Amount: amount, UpdatedAt: s.now().UTC()}
	if err := s.repo.Save(ctx, b); err != nil {
		return Balance{}, fmt.Errorf("save balance: %w", err)
	}
	return b, nil
}

// List returns the real balances registered for a period.
func (s *Service) List(ctx context.Context, periodStr string) ([]Balance, error) {
	p, err := period.Parse(periodStr)
	if err != nil {
		return nil, invalid(err)
	}
	return s.repo.ListByPeriod(ctx, p)
}

// Delete removes the real balance of a currency in a period.
func (s *Service) Delete(ctx context.Context, periodStr, currencyStr string) error {
	p, err := period.Parse(periodStr)
	if err != nil {
		return invalid(err)
	}
	currency, err := money.ParseCurrency(currencyStr)
	if err != nil {
		return invalid(err)
	}
	return s.repo.Delete(ctx, p, currency)
}

func invalid(err error) error {
	return fmt.Errorf("%w: %w", ErrInvalidInput, err)
}
