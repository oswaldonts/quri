package sqlite

import (
	"context"
	"testing"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/balance"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/income"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/money"
)

func TestIncomeListAndContribution(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	repo := NewIncomeRepository(db)

	salary := testIncome(t, "inc-1")
	contribution := income.Income{ID: "inc-2", Period: july, Amount: mustMoney(t, "108.00", money.PEN),
		Kind: income.KindExtra, CategoryID: "loans", ReceivedOn: now, CreatedAt: now}
	for _, in := range []income.Income{salary, contribution} {
		if err := repo.Save(ctx, in); err != nil {
			t.Fatalf("Save %s: %v", in.ID, err)
		}
	}

	got, err := repo.ListByPeriod(ctx, july)
	if err != nil || len(got) != 2 {
		t.Fatalf("ListByPeriod = %d incomes, %v; want 2", len(got), err)
	}
	byID := map[string]income.Income{got[0].ID: got[0], got[1].ID: got[1]}
	if c := byID["inc-2"]; c.CategoryID != "loans" || c.Amount.String() != "108.00" || c.Kind != income.KindExtra {
		t.Errorf("contribution round trip = %+v", c)
	}
	if s := byID["inc-1"]; s.CategoryID != "" || s.Amount.String() != "1330.00" {
		t.Errorf("salary round trip = %+v", s)
	}

	// A contribution counts as an entry: the category can't leave the period.
	if has, _ := NewCategoryRepository(db).HasEntries(ctx, july, "loans"); !has {
		t.Error("HasEntries(loans) = false, want true because of the contribution")
	}
}

func TestBalanceRepository(t *testing.T) {
	ctx := context.Background()
	repo := NewBalanceRepository(openTestDB(t))

	for _, b := range []balance.Balance{
		{Period: july, Amount: mustMoney(t, "400.00", money.PEN), UpdatedAt: now},
		{Period: july, Amount: mustMoney(t, "415.50", money.PEN), UpdatedAt: now}, // replaces the previous one
		{Period: july, Amount: mustMoney(t, "-20.00", money.USD), UpdatedAt: now},
	} {
		if err := repo.Save(ctx, b); err != nil {
			t.Fatalf("Save: %v", err)
		}
	}

	got, err := repo.ListByPeriod(ctx, july)
	if err != nil || len(got) != 2 {
		t.Fatalf("ListByPeriod = %+v, %v; want 2", got, err)
	}
	if got[0].Amount.Currency() != money.PEN || got[0].Amount.String() != "415.50" ||
		got[1].Amount.Currency() != money.USD || got[1].Amount.String() != "-20.00" {
		t.Errorf("ListByPeriod = %s %s, %s %s", got[0].Amount.Currency(), got[0].Amount, got[1].Amount.Currency(), got[1].Amount)
	}
}
