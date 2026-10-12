package summary

import (
	"context"
	"fmt"
	"slices"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/balance"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/expense"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/income"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/money"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/period"
)

// CarrySource tells where the money carried into a period comes from.
type CarrySource string

const (
	// CarryFromReal: the real balance registered at the end of the previous period.
	CarryFromReal CarrySource = "real"
	// CarryFromAvailable: the available money computed for the previous period
	// (no real balance was registered).
	CarryFromAvailable CarrySource = "available"
)

// Carry is what a currency brings from the previous period.
type Carry struct {
	Amount money.Money
	From   CarrySource
}

// carries holds the carry of each currency; currencies that carry nothing are absent.
type carries map[money.Currency]Carry

// PeriodBounds tells where the history starts.
type PeriodBounds interface {
	// EarliestPeriod returns the first period with incomes, expenses or balances;
	// ok is false when there is no data at all.
	EarliestPeriod(ctx context.Context) (p period.Period, ok bool, err error)
}

// carryInto computes what every currency carries into p, walking month by month
// from the first period with data: each month's leftover (its real balance if
// registered, otherwise its available money) becomes the next month's carry.
func (s *Service) carryInto(ctx context.Context, p period.Period, view View) (carries, error) {
	carry := carries{}
	first, ok, err := s.bounds.EarliestPeriod(ctx)
	if err != nil {
		return nil, fmt.Errorf("earliest period: %w", err)
	}
	if !ok || first.Compare(p) >= 0 {
		return carry, nil
	}
	for q := first; q.Compare(p) < 0; q = q.Next() {
		incomes, expenses, balances, err := s.movements(ctx, q, view)
		if err != nil {
			return nil, err
		}
		carry = nextCarries(buildAvailable(incomes, expenses, carry), balances)
	}
	return carry, nil
}

// movements reads what changes the money of a period, as seen from the view.
func (s *Service) movements(ctx context.Context, p period.Period, view View) ([]income.Income, []expense.Expense, []balance.Balance, error) {
	incomes, err := s.incomes.ListByPeriod(ctx, p)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("list incomes: %w", err)
	}
	expenses, err := s.expenses.ListByPeriod(ctx, p)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("list expenses: %w", err)
	}
	balances, err := s.balances.ListByPeriod(ctx, p)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("list balances: %w", err)
	}
	expenses = slices.DeleteFunc(slices.Clone(expenses), func(e expense.Expense) bool { return e.Disabled })
	incomes, expenses = view.filter(incomes, expenses)
	return incomes, expenses, balances, nil
}

// nextCarries turns a period's available money and real balances into the carry of
// the following period. A real balance wins over the computed available money.
func nextCarries(available []Available, balances []balance.Balance) carries {
	next := carries{}
	for _, a := range available {
		next[a.Currency] = Carry{Amount: a.Available, From: CarryFromAvailable}
	}
	for _, b := range balances {
		next[b.Amount.Currency()] = Carry{Amount: b.Amount, From: CarryFromReal}
	}
	return withoutZeros(next)
}

// carriesFromSummary is nextCarries for a period whose full summary is already built.
func carriesFromSummary(s Summary) carries {
	next := carries{}
	for _, cs := range s.Currencies {
		if cs.Real != nil {
			next[cs.Currency] = Carry{Amount: *cs.Real, From: CarryFromReal}
		} else {
			next[cs.Currency] = Carry{Amount: cs.Available, From: CarryFromAvailable}
		}
	}
	return withoutZeros(next)
}

// withoutZeros drops currencies that carry nothing, so a currency used once (e.g.
// on a trip) doesn't show up with zeros in every following month.
func withoutZeros(c carries) carries {
	for currency, carry := range c {
		if carry.Amount.Amount() == 0 {
			delete(c, currency)
		}
	}
	return c
}
