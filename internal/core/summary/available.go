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

// Available is what is left of a period in one currency: carried over + income − spent.
type Available struct {
	Currency    money.Currency
	CarriedOver money.Money // brought from the previous period
	CarriedFrom CarrySource // "" when nothing is carried
	Income      money.Money
	Spent       money.Money
	Available   money.Money
}

// Available returns, per currency, how much money is left in a period. It is the
// lightweight version of Get: it only reads incomes and expenses.
func (s *Service) Available(ctx context.Context, periodStr, viewStr string) ([]Available, error) {
	p, err := period.Parse(periodStr)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidInput, err)
	}
	view, err := ParseView(viewStr)
	if err != nil {
		return nil, err
	}
	carry, err := s.carryInto(ctx, p, view)
	if err != nil {
		return nil, err
	}
	incomes, expenses, _, err := s.movements(ctx, p, view)
	if err != nil {
		return nil, err
	}
	return buildAvailable(incomes, expenses, carry), nil
}

// buildAvailable is pure: one entry per currency with activity, sorted by currency.
func buildAvailable(incomes []income.Income, expenses []expense.Expense, carry carries) []Available {
	byCurrency := map[money.Currency]*Available{}
	get := func(c money.Currency) *Available {
		a := byCurrency[c]
		if a == nil {
			zero := money.Zero(c)
			a = &Available{Currency: c, CarriedOver: zero, Income: zero, Spent: zero, Available: zero}
			byCurrency[c] = a
		}
		return a
	}

	for c, cr := range carry {
		a := get(c)
		a.CarriedOver, a.CarriedFrom = cr.Amount, cr.From
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
		a.Available = sub(add(a.CarriedOver, a.Income), a.Spent)
		out = append(out, *a)
	}
	return out
}
