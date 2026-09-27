package sqlite

import (
	"context"
	"testing"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/balance"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/money"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/period"
)

func TestEarliestPeriod(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	bounds := NewPeriodBounds(db)

	if _, ok, err := bounds.EarliestPeriod(ctx); ok || err != nil {
		t.Fatalf("empty database: ok = %v, err = %v; want false, nil", ok, err)
	}

	if err := NewIncomeRepository(db).Save(ctx, testIncome(t, "inc-1")); err != nil { // July
		t.Fatal(err)
	}
	june, _ := period.Parse("2026-06")
	if err := NewBalanceRepository(db).Save(ctx, balance.Balance{Period: june, Amount: mustMoney(t, "100", money.PEN), UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}

	got, ok, err := bounds.EarliestPeriod(ctx)
	if err != nil || !ok || got != june {
		t.Errorf("EarliestPeriod = %s, %v, %v; want 2026-06 (the balance)", got, ok, err)
	}
}
