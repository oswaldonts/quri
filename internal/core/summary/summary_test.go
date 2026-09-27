package summary

import (
	"testing"
	"time"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/balance"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/budget"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/category"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/expense"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/income"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/money"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/period"
)

var (
	july, _ = period.Parse("2026-07")

	catalog = []category.Category{
		{ID: "market", Name: "Mercado", Group: category.GroupNeeds},
		{ID: "health-insurance", Name: "Seguro médico", Group: category.GroupNeeds, Committed: true},
		{ID: "loans", Name: "Créditos", Group: category.GroupObligations, Committed: true},
		{ID: "leisure", Name: "Ocio", Group: category.GroupLifestyle},
		{ID: "exchange", Name: "Cambio", Group: category.GroupSystem},
	}
)

func m(t *testing.T, amount string, c money.Currency) money.Money {
	t.Helper()
	v, err := money.Parse(amount, c)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func exp(t *testing.T, cat, amount string, c money.Currency, desc string, tags ...string) expense.Expense {
	return expense.Expense{Period: july, CategoryID: cat, Amount: m(t, amount, c), Description: desc, Tags: tags, SpentOn: time.Now()}
}

// testData mirrors the user's July spreadsheet, reduced to a few rows.
func testData(t *testing.T) ([]income.Income, []expense.Expense, []balance.Balance) {
	incomes := []income.Income{
		{Kind: income.KindSalary, Amount: m(t, "1330.00", money.USD)},
		{Kind: income.KindExchange, Amount: m(t, "3562.00", money.PEN)},
		{Kind: income.KindExtra, Amount: m(t, "108.00", money.PEN), CategoryID: "loans"},
	}
	expenses := []expense.Expense{
		{CategoryID: "exchange", Amount: m(t, "1096.00", money.USD), ExchangeID: "ex-1"},
		exp(t, "leisure", "48.50", money.USD, "Juego"),
		exp(t, "health-insurance", "1000.00", money.PEN, "Seguro"),
		exp(t, "loans", "462.00", money.PEN, "Cuota crédito"),
		exp(t, "market", "27.00", money.PEN, "Chocolate Milky", "dulces"),
		exp(t, "market", "25.00", money.PEN, "chocolate  milky", "dulces"),
		exp(t, "market", "8.50", money.PEN, "Pringles", "dulces", "snacks"),
		exp(t, "market", "37.00", money.PEN, "Queso", "lácteos"),
	}
	balances := []balance.Balance{
		{Period: july, Amount: m(t, "2104.50", money.PEN)},
	}
	return incomes, expenses, balances
}

func TestBuild(t *testing.T) {
	incomes, expenses, balances := testData(t)
	s := build(july, entries{incomes: incomes, expenses: expenses, balances: balances}, catalog, nil)

	if len(s.Currencies) != 2 || s.Currencies[0].Currency != money.PEN || s.Currencies[1].Currency != money.USD {
		t.Fatalf("currencies = %v, want PEN then USD", s.Currencies)
	}
	pen, usd := s.Currencies[0], s.Currencies[1]

	check := func(name string, got money.Money, want string) {
		t.Helper()
		if got.String() != want {
			t.Errorf("%s = %s, want %s", name, got, want)
		}
	}

	// Income by kind; the contribution counts as extra.
	check("PEN salary", pen.Income.Salary, "0.00")
	check("PEN exchange", pen.Income.Exchange, "3562.00")
	check("PEN extra", pen.Income.Extra, "108.00")
	check("PEN income total", pen.Income.Total, "3670.00")

	// Spent: 1000 + 462 + 27 + 25 + 8.50 + 37 = 1559.50; available = 3670 − 1559.50.
	check("PEN spent", pen.Spent, "1559.50")
	check("PEN available", pen.Available, "2110.50")
	check("PEN committed", pen.Committed, "1462.00")
	check("PEN discretionary", pen.Discretionary, "97.50")

	// Real 2104.50 → difference 6.00: 6 PEN are missing from the registered expenses.
	if pen.Real == nil || pen.Difference == nil {
		t.Fatal("PEN real and difference must be set")
	}
	check("PEN difference", *pen.Difference, "6.00")

	// Categories by amount spent; the contribution to Créditos has its own section.
	if first := pen.Categories[0]; first.Category.Name != "Seguro médico" {
		t.Errorf("top category = %s, want Seguro médico", first.Category.Name)
	}
	if len(pen.Contributions) != 1 || pen.Contributions[0].Category.ID != "loans" || pen.Contributions[0].Count != 1 {
		t.Fatalf("contributions = %+v, want one to Créditos", pen.Contributions)
	}
	check("Créditos contribution", pen.Contributions[0].Amount, "108.00")
	for _, ct := range pen.Categories {
		if ct.Category.ID == "loans" {
			check("Créditos spent", ct.Spent, "462.00")
		}
		if ct.Category.ID == "market" && ct.Count != 4 {
			t.Errorf("Mercado count = %d, want 4", ct.Count)
		}
	}

	// Tags: an expense with two tags counts in both.
	if len(pen.Tags) != 3 || pen.Tags[0].Tag != "dulces" {
		t.Fatalf("tags = %+v, want dulces first", pen.Tags)
	}
	check("dulces", pen.Tags[0].Spent, "60.50")
	if pen.Tags[0].Count != 3 {
		t.Errorf("dulces count = %d, want 3", pen.Tags[0].Count)
	}

	// Items: "Chocolate Milky" and "chocolate  milky" are the same item.
	top := pen.TopItems[0]
	if top.Description != "Seguro" {
		t.Errorf("top item = %s, want Seguro", top.Description)
	}
	for _, it := range pen.TopItems {
		if it.Description == "Chocolate Milky" {
			check("Chocolate Milky", it.Spent, "52.00")
			if it.Count != 2 {
				t.Errorf("Chocolate Milky count = %d, want 2", it.Count)
			}
		}
	}

	// USD: the exchange is tracked apart and kept out of the item ranking; no real balance.
	check("USD exchanged", usd.Exchanged, "1096.00")
	check("USD available", usd.Available, "185.50")
	if usd.Real != nil || usd.Difference != nil {
		t.Error("USD has no real balance: real and difference must be nil")
	}
	if len(usd.TopItems) != 1 || usd.TopItems[0].Description != "Juego" {
		t.Errorf("USD items = %+v, want only Juego", usd.TopItems)
	}
	if usd.Groups[0].Group != category.GroupSystem {
		t.Errorf("USD top group = %s, want system (Cambio)", usd.Groups[0].Group)
	}
}

func TestBuildTopItemsLimit(t *testing.T) {
	var expenses []expense.Expense
	for i := range 15 {
		expenses = append(expenses, exp(t, "market", "1", money.PEN, string(rune('a'+i))))
	}
	s := build(july, entries{expenses: expenses}, catalog, nil)
	if n := len(s.Currencies[0].TopItems); n != maxTopItems {
		t.Errorf("top items = %d, want %d", n, maxTopItems)
	}
}

func TestBuildEmptyPeriod(t *testing.T) {
	if s := build(july, entries{}, catalog, nil); len(s.Currencies) != 0 {
		t.Errorf("empty period: currencies = %v, want none", s.Currencies)
	}
}

func TestBuildBudgets(t *testing.T) {
	_, expenses, _ := testData(t)
	budgets := []budget.Budget{
		{CategoryID: "market", Amount: m(t, "80.00", money.PEN)},  // spent 97.50 → over budget
		{CategoryID: "leisure", Amount: m(t, "30.00", money.PEN)}, // no PEN expenses yet
		{CategoryID: "leisure", Amount: m(t, "50.00", money.USD)}, // spent 48.50
	}
	s := build(july, entries{expenses: expenses, budgets: budgets}, catalog, nil)
	pen, usd := s.Currencies[0], s.Currencies[1]

	find := func(cs CurrencySummary, id string) CategoryTotal {
		t.Helper()
		for _, ct := range cs.Categories {
			if ct.Category.ID == id {
				return ct
			}
		}
		t.Fatalf("category %s not in %s", id, cs.Currency)
		return CategoryTotal{}
	}

	market := find(pen, "market")
	if market.Budget == nil || market.Remaining == nil || market.Remaining.String() != "-17.50" {
		t.Errorf("Mercado budget/remaining = %v/%v, want 80.00/-17.50", market.Budget, market.Remaining)
	}
	// A budgeted category shows up even without expenses.
	if leisure := find(pen, "leisure"); leisure.Remaining == nil || leisure.Remaining.String() != "30.00" {
		t.Errorf("Ocio PEN remaining = %v, want 30.00", leisure.Remaining)
	}
	if leisure := find(usd, "leisure"); leisure.Remaining == nil || leisure.Remaining.String() != "1.50" {
		t.Errorf("Ocio USD remaining = %v, want 1.50", leisure.Remaining)
	}
	// Categories without a budget have none.
	if ins := find(pen, "health-insurance"); ins.Budget != nil || ins.Remaining != nil {
		t.Errorf("Seguro médico has no budget, got %v/%v", ins.Budget, ins.Remaining)
	}
	if pen.Budgeted == nil || pen.Budgeted.String() != "110.00" {
		t.Errorf("PEN budgeted = %v, want 110.00", pen.Budgeted)
	}
}

func TestContributionWithoutExpensesIsNotACategory(t *testing.T) {
	incomes := []income.Income{{Kind: income.KindExtra, Amount: m(t, "118.00", money.PEN), CategoryID: "loans"}}
	s := build(july, entries{incomes: incomes}, catalog, nil)
	pen := s.Currencies[0]

	if len(pen.Categories) != 0 {
		t.Errorf("categories = %+v, want none: a contribution is not an expense", pen.Categories)
	}
	if len(pen.Contributions) != 1 || pen.Contributions[0].Amount.String() != "118.00" {
		t.Errorf("contributions = %+v", pen.Contributions)
	}
}

func TestCurrencyOfEmpty(t *testing.T) {
	empty := currencyOf(Summary{}, money.USD)
	if empty.Currency != money.USD || empty.Spent.String() != "0.00" || empty.Committed.String() != "0.00" ||
		empty.Income.Total.String() != "0.00" || empty.Available.String() != "0.00" {
		t.Errorf("empty block = %+v, want every amount 0.00 in USD", empty)
	}
}
