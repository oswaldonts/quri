package sqlite

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/expense"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/money"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/period"
)

func TestExpenseRepository(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	repo := NewExpenseRepository(db)
	categories := NewCategoryRepository(db)

	later := expense.Expense{ID: "e2", Period: july, CategoryID: "market", Amount: mustMoney(t, "20.00", money.PEN),
		SpentOn: now.AddDate(0, 0, 5), CreatedAt: now}
	earlier := expense.Expense{ID: "e1", Period: july, CategoryID: "market", Amount: mustMoney(t, "45.50", money.PEN),
		Description: "Mercado", Tags: []string{"dulces", "lácteos"}, SpentOn: now, CreatedAt: now}
	for _, e := range []expense.Expense{later, earlier} {
		if err := repo.Save(ctx, e); err != nil {
			t.Fatalf("Save %s: %v", e.ID, err)
		}
	}

	got, err := repo.ListByPeriod(ctx, july)
	if err != nil {
		t.Fatalf("ListByPeriod: %v", err)
	}
	if len(got) != 2 || got[0].ID != "e1" || got[1].ID != "e2" {
		t.Fatalf("ListByPeriod = %+v, want e1 then e2 (by date)", got)
	}
	if e := got[0]; e.Amount.String() != "45.50" || e.Amount.Currency() != money.PEN ||
		e.Description != "Mercado" || !e.SpentOn.Equal(now.Truncate(24*time.Hour)) || e.ExchangeID != "" {
		t.Errorf("round trip = %+v", e)
	}
	if !slices.Equal(got[0].Tags, []string{"dulces", "lácteos"}) || len(got[1].Tags) != 0 {
		t.Errorf("tags = %q and %q, want [dulces lácteos] and none", got[0].Tags, got[1].Tags)
	}

	if has, _ := categories.HasEntries(ctx, july, "market"); !has {
		t.Error("HasEntries(market) = false, want true")
	}
	if has, _ := categories.HasEntries(ctx, july, "travel"); has {
		t.Error("HasEntries(travel) = true, want false")
	}
	aug, _ := period.Parse("2026-08")
	if has, _ := categories.HasEntries(ctx, aug, "market"); has {
		t.Error("HasExpenses in another period = true, want false")
	}

	// The foreign key rejects unknown categories.
	bad := earlier
	bad.ID, bad.CategoryID = "e3", "nope"
	if err := repo.Save(ctx, bad); err == nil {
		t.Error("unknown category must fail")
	}

	// A failing tag insert rolls the whole expense back.
	dupTag := earlier
	dupTag.ID, dupTag.Tags = "e4", []string{"x", "x"}
	if err := repo.Save(ctx, dupTag); err == nil {
		t.Fatal("duplicated tag must fail")
	}
	if n := count(t, db, "expenses"); n != 2 {
		t.Errorf("expenses = %d, want 2: the failed one was not rolled back", n)
	}
}

func TestExpenseRepositorySaveAllIsAtomic(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	repo := NewExpenseRepository(db)

	ok := expense.Expense{ID: "a", Period: july, CategoryID: "market", Amount: mustMoney(t, "1", money.PEN), SpentOn: now, CreatedAt: now}
	bad := ok
	bad.ID, bad.CategoryID = "b", "nope" // violates the foreign key

	if err := repo.SaveAll(ctx, []expense.Expense{ok, bad}); err == nil {
		t.Fatal("SaveAll with an invalid expense must fail")
	}
	if n := count(t, db, "expenses"); n != 0 {
		t.Errorf("expenses = %d, want 0: the valid one must be rolled back too", n)
	}

	second := ok
	second.ID = "c"
	if err := repo.SaveAll(ctx, []expense.Expense{ok, second}); err != nil {
		t.Fatalf("SaveAll: %v", err)
	}
	if n := count(t, db, "expenses"); n != 2 {
		t.Errorf("expenses = %d, want 2", n)
	}
}
