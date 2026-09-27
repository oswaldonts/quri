package sqlite

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/balance"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/exchange"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/expense"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/income"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/money"
)

func TestIncomeGetUpdateDelete(t *testing.T) {
	ctx := context.Background()
	repo := NewIncomeRepository(openTestDB(t))
	in := testIncome(t, "inc-1")
	if err := repo.Save(ctx, in); err != nil {
		t.Fatal(err)
	}

	in.Amount, in.Kind, in.CategoryID, in.Description = mustMoney(t, "108.00", money.PEN), income.KindExtra, "loans", "Aporte"
	if err := repo.Update(ctx, in); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, err := repo.Get(ctx, "inc-1")
	if err != nil || got.Amount.String() != "108.00" || got.Amount.Currency() != money.PEN ||
		got.Kind != income.KindExtra || got.CategoryID != "loans" || got.Description != "Aporte" {
		t.Errorf("Get after Update = %+v, %v", got, err)
	}

	if err := repo.Delete(ctx, "inc-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	for name, err := range map[string]error{
		"get":    func() error { _, err := repo.Get(ctx, "inc-1"); return err }(),
		"update": repo.Update(ctx, in),
		"delete": repo.Delete(ctx, "inc-1"),
	} {
		if !errors.Is(err, income.ErrNotFound) {
			t.Errorf("%s after delete: err = %v, want %v", name, err, income.ErrNotFound)
		}
	}
}

func TestExpenseGetUpdateDelete(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	repo := NewExpenseRepository(db)
	e := expense.Expense{ID: "e1", Period: july, CategoryID: "market", Amount: mustMoney(t, "45.50", money.PEN),
		Description: "Queso", Tags: []string{"lácteos", "mercado"}, SpentOn: now, CreatedAt: now}
	if err := repo.Save(ctx, e); err != nil {
		t.Fatal(err)
	}

	got, err := repo.Get(ctx, "e1")
	if err != nil || !slices.Equal(got.Tags, []string{"lácteos", "mercado"}) {
		t.Fatalf("Get = %+v, %v", got, err)
	}

	e.Amount, e.CategoryID, e.Tags = mustMoney(t, "54.50", money.PEN), "leisure", []string{"salidas"}
	if err := repo.Update(ctx, e); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ = repo.Get(ctx, "e1")
	if got.Amount.String() != "54.50" || got.CategoryID != "leisure" || !slices.Equal(got.Tags, []string{"salidas"}) {
		t.Errorf("Get after Update = %+v", got)
	}

	if err := repo.Delete(ctx, "e1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if n := count(t, db, "expense_tags"); n != 0 {
		t.Errorf("expense_tags = %d, want 0 (ON DELETE CASCADE)", n)
	}
	if _, err := repo.Get(ctx, "e1"); !errors.Is(err, expense.ErrNotFound) {
		t.Errorf("Get after Delete: err = %v, want %v", err, expense.ErrNotFound)
	}
	if err := repo.Update(ctx, e); !errors.Is(err, expense.ErrNotFound) {
		t.Errorf("Update after Delete: err = %v, want %v", err, expense.ErrNotFound)
	}
}

func TestExchangeListAndDelete(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	repo := NewExchangeRepository(db)
	ex, inc, out := testExchange(t, "ex-1", "inc-1")
	if err := repo.Save(ctx, ex, inc, out); err != nil {
		t.Fatal(err)
	}

	list, err := repo.ListByPeriod(ctx, july)
	if err != nil || len(list) != 1 || list[0].From.String() != "1096.00" || list[0].Rate.String() != "3.25" {
		t.Fatalf("ListByPeriod = %+v, %v", list, err)
	}
	got, err := repo.Get(ctx, "ex-1")
	if err != nil || got.IncomeID != "inc-1" {
		t.Fatalf("Get = %+v, %v", got, err)
	}

	if err := repo.Delete(ctx, got); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	for _, table := range []string{"exchanges", "incomes", "expenses"} {
		if n := count(t, db, table); n != 0 {
			t.Errorf("%s = %d after deleting the exchange, want 0", table, n)
		}
	}
	if _, err := repo.Get(ctx, "ex-1"); !errors.Is(err, exchange.ErrNotFound) {
		t.Errorf("Get after Delete: err = %v, want %v", err, exchange.ErrNotFound)
	}
}

func TestBalanceDelete(t *testing.T) {
	ctx := context.Background()
	repo := NewBalanceRepository(openTestDB(t))
	_ = repo.Save(ctx, balance.Balance{Period: july, Amount: mustMoney(t, "10", money.PEN), UpdatedAt: now})

	if err := repo.Delete(ctx, july, money.PEN); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := repo.Delete(ctx, july, money.PEN); !errors.Is(err, balance.ErrNotFound) {
		t.Errorf("second Delete: err = %v, want %v", err, balance.ErrNotFound)
	}
}
