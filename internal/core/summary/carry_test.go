package summary

import (
	"context"
	"testing"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/balance"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/budget"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/category"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/expense"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/income"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/money"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/period"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/status"
)

// store is an in-test implementation of every reader the service needs.
type store struct {
	incomes  map[period.Period][]income.Income
	expenses map[period.Period][]expense.Expense
	balances map[period.Period][]balance.Balance
}

func newStore() *store {
	return &store{
		incomes:  map[period.Period][]income.Income{},
		expenses: map[period.Period][]expense.Expense{},
		balances: map[period.Period][]balance.Balance{},
	}
}

type incomeReader struct{ *store }
type expenseReader struct{ *store }
type balanceReader struct{ *store }
type categoryReader struct{}
type budgetReader struct{}
type bounds struct{ *store }

func (r incomeReader) ListByPeriod(_ context.Context, p period.Period) ([]income.Income, error) {
	return r.incomes[p], nil
}
func (r expenseReader) ListByPeriod(_ context.Context, p period.Period) ([]expense.Expense, error) {
	return r.expenses[p], nil
}
func (r balanceReader) ListByPeriod(_ context.Context, p period.Period) ([]balance.Balance, error) {
	return r.balances[p], nil
}
func (categoryReader) List(context.Context, bool) ([]category.Category, error) { return catalog, nil }
func (budgetReader) ListByPeriod(context.Context, period.Period) ([]budget.Budget, error) {
	return nil, nil
}

func (b bounds) EarliestPeriod(context.Context) (period.Period, bool, error) {
	var first period.Period
	found := false
	for p := range b.periods() {
		if !found || p.Compare(first) < 0 {
			first, found = p, true
		}
	}
	return first, found, nil
}

func (s *store) periods() map[period.Period]int {
	all := map[period.Period]int{}
	for p := range s.incomes {
		all[p]++
	}
	for p := range s.expenses {
		all[p]++
	}
	for p := range s.balances {
		all[p]++
	}
	return all
}

func newCarryService(s *store) *Service {
	return NewService(incomeReader{s}, expenseReader{s}, balanceReader{s}, categoryReader{}, budgetReader{}, bounds{s})
}

func mustP(t *testing.T, s string) period.Period {
	t.Helper()
	p, err := period.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// august reproduces the user's first month: 130 USD and 3018 PEN left.
func august(t *testing.T, s *store) {
	aug := mustP(t, "2026-08")
	s.incomes[aug] = []income.Income{
		{Kind: income.KindSalary, Amount: m(t, "1330.00", money.USD)},
		{Kind: income.KindExchange, Amount: m(t, "3900.00", money.PEN)},
		{Kind: income.KindExtra, Amount: m(t, "118.00", money.PEN), CategoryID: "loans"},
	}
	s.expenses[aug] = []expense.Expense{
		{CategoryID: "exchange", Amount: m(t, "1200.00", money.USD), ExchangeID: "ex-1"},
		{CategoryID: "health-insurance", Amount: m(t, "1000.00", money.PEN)},
	}
}

func availableOf(t *testing.T, svc *Service, p string) map[money.Currency]Available {
	t.Helper()
	as, err := svc.Available(context.Background(), p, "")
	if err != nil {
		t.Fatalf("Available(%s): %v", p, err)
	}
	out := map[money.Currency]Available{}
	for _, a := range as {
		out[a.Currency] = a
	}
	return out
}

func TestCarryFromAvailable(t *testing.T) {
	s := newStore()
	august(t, s)
	svc := newCarryService(s)

	// The first month carries nothing.
	aug := availableOf(t, svc, "2026-08")
	if aug[money.PEN].CarriedFrom != "" || aug[money.PEN].CarriedOver.String() != "0.00" {
		t.Errorf("August carry = %+v, want none", aug[money.PEN])
	}

	// September starts with August's leftover, even without entries of its own.
	sep := availableOf(t, svc, "2026-09")
	if pen := sep[money.PEN]; pen.CarriedOver.String() != "3018.00" || pen.CarriedFrom != CarryFromAvailable || pen.Available.String() != "3018.00" {
		t.Errorf("September PEN = %+v, want 3018.00 carried from available", pen)
	}
	if usd := sep[money.USD]; usd.CarriedOver.String() != "130.00" || usd.Available.String() != "130.00" {
		t.Errorf("September USD = %+v, want 130.00", usd)
	}

	// With September's own entries, the carry adds up with them.
	s.incomes[mustP(t, "2026-09")] = []income.Income{{Kind: income.KindSalary, Amount: m(t, "1330.00", money.USD)}}
	s.expenses[mustP(t, "2026-09")] = []expense.Expense{{CategoryID: "market", Amount: m(t, "200.00", money.PEN)}}
	sep = availableOf(t, svc, "2026-09")
	if sep[money.PEN].Available.String() != "2818.00" || sep[money.USD].Available.String() != "1460.00" {
		t.Errorf("September = PEN %s, USD %s; want 2818.00 and 1460.00", sep[money.PEN].Available, sep[money.USD].Available)
	}

	// October chains: it carries September's leftover, which already included August.
	oct := availableOf(t, svc, "2026-10")
	if oct[money.PEN].CarriedOver.String() != "2818.00" || oct[money.USD].CarriedOver.String() != "1460.00" {
		t.Errorf("October carry = PEN %s, USD %s; want 2818.00 and 1460.00", oct[money.PEN].CarriedOver, oct[money.USD].CarriedOver)
	}
}

func TestCarryPrefersRealBalance(t *testing.T) {
	s := newStore()
	august(t, s)
	// At the end of August the bank said 3000 PEN, not the computed 3018.
	s.balances[mustP(t, "2026-08")] = []balance.Balance{{Amount: m(t, "3000.00", money.PEN)}}
	svc := newCarryService(s)

	sep := availableOf(t, svc, "2026-09")
	if pen := sep[money.PEN]; pen.CarriedOver.String() != "3000.00" || pen.CarriedFrom != CarryFromReal {
		t.Errorf("September PEN = %+v, want 3000.00 carried from real", pen)
	}
	if usd := sep[money.USD]; usd.CarriedFrom != CarryFromAvailable {
		t.Errorf("September USD carried from %q, want available (no real USD balance)", usd.CarriedFrom)
	}

	// The summary shows the same figures.
	sum, err := svc.Get(context.Background(), "2026-09", "")
	if err != nil {
		t.Fatal(err)
	}
	pen := currencyOf(sum, money.PEN)
	if pen.CarriedOver.String() != "3000.00" || pen.CarriedFrom != CarryFromReal || pen.Available.String() != "3000.00" {
		t.Errorf("September summary PEN = carried %s from %q, available %s", pen.CarriedOver, pen.CarriedFrom, pen.Available)
	}
}

func TestCarryDropsZeros(t *testing.T) {
	s := newStore()
	aug := mustP(t, "2026-08")
	// A trip: everything received in ARS was spent.
	s.incomes[aug] = []income.Income{{Kind: income.KindExchange, Amount: m(t, "230000.00", money.ARS)}}
	s.expenses[aug] = []expense.Expense{{CategoryID: "travel", Amount: m(t, "230000.00", money.ARS)}}
	svc := newCarryService(s)

	if _, ok := availableOf(t, svc, "2026-09")[money.ARS]; ok {
		t.Error("ARS carries 0 into September and must not appear")
	}
}

func TestTrendsIncludeCarry(t *testing.T) {
	s := newStore()
	august(t, s)
	s.expenses[mustP(t, "2026-09")] = []expense.Expense{{CategoryID: "market", Amount: m(t, "200.00", money.PEN)}}
	svc := newCarryService(s)

	tr, err := svc.Trends(context.Background(), TrendsInput{From: "2026-08", To: "2026-10", Currency: "PEN"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"3018.00", "2818.00", "2818.00"} // Aug, Sep (carry − 200), Oct (only carry)
	for i, p := range tr.Periods {
		if p.Available.String() != want[i] {
			t.Errorf("%s available = %s, want %s", p.Period, p.Available, want[i])
		}
	}
	// Income stays what was really received: the carry is not counted as income.
	if tr.Periods[1].Income.String() != "0.00" {
		t.Errorf("September income = %s, want 0.00", tr.Periods[1].Income)
	}
}

func TestProjectedView(t *testing.T) {
	s := newStore()
	august(t, s)
	sep := mustP(t, "2026-09")
	// Planning September: salary and an exchange expected, insurance expected, groceries already done.
	s.incomes[sep] = []income.Income{
		{Kind: income.KindSalary, Amount: m(t, "1330.00", money.USD), Status: status.Planned},
	}
	s.expenses[sep] = []expense.Expense{
		{CategoryID: "health-insurance", Amount: m(t, "1000.00", money.PEN), Status: status.Planned},
		{CategoryID: "market", Amount: m(t, "200.00", money.PEN), Status: status.Confirmed},
	}
	svc := newCarryService(s)
	ctx := context.Background()

	confirmed, _ := svc.Available(ctx, "2026-09", "confirmed")
	projected, _ := svc.Available(ctx, "2026-09", "projected")
	byCurrency := func(as []Available) map[money.Currency]string {
		out := map[money.Currency]string{}
		for _, a := range as {
			out[a.Currency] = a.Available.String()
		}
		return out
	}
	// Confirmed: 3018 − 200 PEN; 130 USD. Projected: also − 1000 PEN and + 1330 USD.
	if got := byCurrency(confirmed); got[money.PEN] != "2818.00" || got[money.USD] != "130.00" {
		t.Errorf("confirmed = %v", got)
	}
	if got := byCurrency(projected); got[money.PEN] != "1818.00" || got[money.USD] != "1460.00" {
		t.Errorf("projected = %v", got)
	}

	// October carries each view's own leftover.
	octConfirmed, _ := svc.Available(ctx, "2026-10", "")
	octProjected, _ := svc.Available(ctx, "2026-10", "projected")
	if byCurrency(octConfirmed)[money.USD] != "130.00" || byCurrency(octProjected)[money.USD] != "1460.00" {
		t.Errorf("October USD confirmed/projected = %s/%s, want 130.00/1460.00",
			byCurrency(octConfirmed)[money.USD], byCurrency(octProjected)[money.USD])
	}

	if _, err := svc.Available(ctx, "2026-09", "maybe"); err == nil {
		t.Error("unknown view must fail")
	}
}
