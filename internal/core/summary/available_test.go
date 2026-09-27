package summary

import (
	"testing"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/expense"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/income"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/money"
)

// TestBuildAvailable mirrors the user's August: salary, one exchange, an extra and an expense.
func TestBuildAvailable(t *testing.T) {
	incomes := []income.Income{
		{Kind: income.KindSalary, Amount: m(t, "1330.00", money.USD)},
		{Kind: income.KindExchange, Amount: m(t, "3900.00", money.PEN)},
		{Kind: income.KindExtra, Amount: m(t, "118.00", money.PEN), CategoryID: "loans"},
	}
	expenses := []expense.Expense{
		{CategoryID: "exchange", Amount: m(t, "1200.00", money.USD), ExchangeID: "ex-1"},
		{CategoryID: "health-insurance", Amount: m(t, "1000.00", money.PEN)},
	}

	got := buildAvailable(incomes, expenses)
	if len(got) != 2 {
		t.Fatalf("got %d currencies, want 2", len(got))
	}
	pen, usd := got[0], got[1]
	if pen.Currency != money.PEN || pen.Income.String() != "4018.00" || pen.Spent.String() != "1000.00" || pen.Available.String() != "3018.00" {
		t.Errorf("PEN = %s − %s = %s, want 4018.00 − 1000.00 = 3018.00", pen.Income, pen.Spent, pen.Available)
	}
	if usd.Currency != money.USD || usd.Available.String() != "130.00" {
		t.Errorf("USD available = %s, want 130.00", usd.Available)
	}

	if empty := buildAvailable(nil, nil); len(empty) != 0 {
		t.Errorf("empty period = %v, want none", empty)
	}
}
