package sqlite

import (
	"context"
	"errors"
	"testing"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/budget"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/money"
)

func TestBudgetRepository(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	repo := NewBudgetRepository(db)

	for _, amount := range []string{"300", "350"} {
		if err := repo.Save(ctx, budget.Budget{Period: july, CategoryID: "market", Amount: mustMoney(t, amount, money.PEN), UpdatedAt: now}); err != nil {
			t.Fatalf("Save: %v", err)
		}
	}
	got, err := repo.ListByPeriod(ctx, july)
	if err != nil || len(got) != 1 || got[0].Amount.String() != "350.00" || got[0].CategoryID != "market" {
		t.Fatalf("ListByPeriod = %+v, %v", got, err)
	}

	// A budget keeps the category in use in the period.
	if has, _ := NewCategoryRepository(db).HasEntries(ctx, july, "market"); !has {
		t.Error("HasEntries(market) = false, want true because of the budget")
	}

	if err := repo.Delete(ctx, july, "market", money.PEN); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := repo.Delete(ctx, july, "market", money.PEN); !errors.Is(err, budget.ErrNotFound) {
		t.Errorf("second Delete: err = %v, want %v", err, budget.ErrNotFound)
	}
}
