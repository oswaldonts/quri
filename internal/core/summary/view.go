package summary

import (
	"fmt"
	"slices"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/expense"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/income"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/status"
)

// View selects which entries a computation includes.
type View string

const (
	// ViewConfirmed counts only what happened. It is the default.
	ViewConfirmed View = "confirmed"
	// ViewProjected also counts planned entries: the plan of a month.
	ViewProjected View = "projected"
)

// ParseView reads a view; an empty string means ViewConfirmed.
func ParseView(s string) (View, error) {
	switch View(s) {
	case "", ViewConfirmed:
		return ViewConfirmed, nil
	case ViewProjected:
		return ViewProjected, nil
	default:
		return "", fmt.Errorf("%w: view %q (want %q or %q)", ErrInvalidInput, s, ViewConfirmed, ViewProjected)
	}
}

// filter drops planned entries from a confirmed view.
func (v View) filter(incomes []income.Income, expenses []expense.Expense) ([]income.Income, []expense.Expense) {
	if v == ViewProjected {
		return incomes, expenses
	}
	incomes = slices.DeleteFunc(slices.Clone(incomes), func(in income.Income) bool { return in.Status == status.Planned })
	expenses = slices.DeleteFunc(slices.Clone(expenses), func(e expense.Expense) bool { return e.Status == status.Planned })
	return incomes, expenses
}
