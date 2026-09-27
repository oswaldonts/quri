package summary

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/expense"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/income"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/money"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/period"
)

// Available is what is left of a period in one currency: income − spent.
type Available struct {
	Currency  money.Currency
	Income    money.Money
	Spent     money.Money
	Available money.Money
}

// Available returns, per currency, how much money is left in a period. It is the
// lightweight version of Get: it only reads incomes and expenses.
func (s *Service) Available(ctx context.Context, periodStr string) ([]Available, error) {
	p, err := period.Parse(periodStr)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidInput, err)
	}
	incomes, err := s.incomes.ListByPeriod(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("list incomes: %w", err)
	}
	expenses, err := s.expenses.ListByPeriod(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("list expenses: %w", err)
	}
	return buildAvailable(incomes, expenses), nil
}

// buildAvailable is pure: one entry per currency with activity, sorted by currency.
func buildAvailable(incomes []income.Income, expenses []expense.Expense) []Available {
	byCurrency := map[money.Currency]*Available{}
	get := func(c money.Currency) *Available {
		a := byCurrency[c]
		if a == nil {
			zero := money.Zero(c)
			a = &Available{Currency: c, Income: zero, Spent: zero, Available: zero}
			byCurrency[c] = a
		}
		return a
	}

	for _, in := range incomes {
		a := get(in.Amount.Currency())
		a.Income = add(a.Income, in.Amount)
	}
	for _, e := range expenses {
		a := get(e.Amount.Currency())
		a.Spent = add(a.Spent, e.Amount)
	}

	out := make([]Available, 0, len(byCurrency))
	for _, c := range slices.Sorted(maps.Keys(byCurrency)) {
		a := byCurrency[c]
		a.Available = sub(a.Income, a.Spent)
		out = append(out, *a)
	}
	return out
}
