// Package budget stores how much is planned to be spent in a category, per period
// and currency. The summary compares it with what was actually spent.
package budget

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/category"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/currency"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/money"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/period"
)

var (
	// ErrInvalidInput wraps every validation error.
	ErrInvalidInput = errors.New("invalid input")
	// ErrNotFound means the category has no budget in that period and currency.
	ErrNotFound = errors.New("budget not found")
)

// Budget is the amount planned for a category in a period and currency.
type Budget struct {
	Period     period.Period
	CategoryID string
	Amount     money.Money
	UpdatedAt  time.Time
}

// Repository stores one budget per period, category and currency.
type Repository interface {
	// Save creates or replaces the budget of its period, category and currency.
	Save(ctx context.Context, b Budget) error
	// ListByPeriod returns the budgets of the period.
	ListByPeriod(ctx context.Context, p period.Period) ([]Budget, error)
	// Delete removes a budget; it returns ErrNotFound if there is none.
	Delete(ctx context.Context, p period.Period, categoryID string, c money.Currency) error
}

// CategoryResolver checks that a category can be used in a period.
type CategoryResolver interface {
	EnsureInPeriod(ctx context.Context, p period.Period, id string) (category.Category, error)
}

// Service holds the budget use cases.
type Service struct {
	repo       Repository
	categories CategoryResolver
	currencies currency.Checker
	now        func() time.Time
}

// NewService returns a Service backed by repo.
func NewService(repo Repository, categories CategoryResolver, currencies currency.Checker) *Service {
	return &Service{repo: repo, categories: categories, currencies: currencies, now: time.Now}
}

// SetInput carries the budget of a category.
type SetInput struct {
	Period     string // YYYY-MM
	CategoryID string // must be used in the period
	Currency   string // ISO 4217 code
	Amount     string // decimal string greater than zero
}

// Set creates or replaces the budget of a category in a period and currency.
func (s *Service) Set(ctx context.Context, in SetInput) (Budget, error) {
	p, err := period.Parse(in.Period)
	if err != nil {
		return Budget{}, invalid(err)
	}
	currency, err := money.ParseCurrency(in.Currency)
	if err != nil {
		return Budget{}, invalid(err)
	}
	if err := s.currencies.Require(ctx, currency); err != nil {
		return Budget{}, err
	}
	amount, err := money.Parse(in.Amount, currency)
	if err != nil {
		return Budget{}, invalid(err)
	}
	if !amount.IsPositive() {
		return Budget{}, invalid(errors.New("amount must be greater than zero"))
	}

	c, err := s.categories.EnsureInPeriod(ctx, p, in.CategoryID)
	if errors.Is(err, category.ErrNotFound) || errors.Is(err, category.ErrNotInPeriod) {
		return Budget{}, invalid(err)
	}
	if err != nil {
		return Budget{}, fmt.Errorf("check category: %w", err)
	}
	if c.IsSystem() {
		return Budget{}, invalid(fmt.Errorf("the system category %s can't have a budget", c.Name))
	}

	b := Budget{Period: p, CategoryID: c.ID, Amount: amount, UpdatedAt: s.now().UTC()}
	if err := s.repo.Save(ctx, b); err != nil {
		return Budget{}, fmt.Errorf("save budget: %w", err)
	}
	return b, nil
}

// List returns the budgets of a period.
func (s *Service) List(ctx context.Context, periodStr string) ([]Budget, error) {
	p, err := period.Parse(periodStr)
	if err != nil {
		return nil, invalid(err)
	}
	return s.repo.ListByPeriod(ctx, p)
}

// Delete removes the budget of a category in a period and currency.
func (s *Service) Delete(ctx context.Context, periodStr, categoryID, currencyStr string) error {
	p, err := period.Parse(periodStr)
	if err != nil {
		return invalid(err)
	}
	currency, err := money.ParseCurrency(currencyStr)
	if err != nil {
		return invalid(err)
	}
	return s.repo.Delete(ctx, p, categoryID, currency)
}

func invalid(err error) error {
	return fmt.Errorf("%w: %w", ErrInvalidInput, err)
}
