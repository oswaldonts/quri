package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/exchange"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/income"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/money"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/period"
)

// openTestDB creates a fresh database file in a temporary directory,
// removed automatically when the test ends.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func count(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func mustMoney(t *testing.T, amount string, c money.Currency) money.Money {
	t.Helper()
	m, err := money.Parse(amount, c)
	if err != nil {
		t.Fatalf("parse money: %v", err)
	}
	return m
}

var (
	july, _ = period.Parse("2026-07")
	now     = time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
)

func testIncome(t *testing.T, id string) income.Income {
	return income.Income{
		ID:         id,
		Period:     july,
		Amount:     mustMoney(t, "1330.00", money.USD),
		Kind:       income.KindSalary,
		ReceivedOn: now,
		CreatedAt:  now,
	}
}

func TestOpenIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	for i := range 2 { // the second Open finds the migrations already applied
		db, err := Open(context.Background(), path)
		if err != nil {
			t.Fatalf("Open #%d: %v", i+1, err)
		}
		_ = db.Close()
	}
}

func TestIncomeRepositorySave(t *testing.T) {
	db := openTestDB(t)
	repo := NewIncomeRepository(db)

	if err := repo.Save(context.Background(), testIncome(t, "inc-1")); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if got := count(t, db, "incomes"); got != 1 {
		t.Errorf("incomes = %d, want 1", got)
	}
	if err := repo.Save(context.Background(), testIncome(t, "inc-1")); err == nil {
		t.Error("saving a duplicated ID must fail")
	}
}

func TestRateRepository(t *testing.T) {
	ctx := context.Background()
	repo := NewRateRepository(openTestDB(t))

	if _, err := repo.FindRate(ctx, july, money.USD, money.PEN); !errors.Is(err, exchange.ErrRateNotFound) {
		t.Fatalf("empty table: err = %v, want %v", err, exchange.ErrRateNotFound)
	}

	for _, r := range []string{"3.25", "3.30"} { // the second save replaces the first
		rate, _ := money.ParseRate(r)
		if err := repo.SaveRate(ctx, exchange.PeriodRate{Period: july, From: money.USD, To: money.PEN, Rate: rate, UpdatedAt: now}); err != nil {
			t.Fatalf("SaveRate(%s): %v", r, err)
		}
	}

	got, err := repo.FindRate(ctx, july, money.USD, money.PEN)
	if err != nil {
		t.Fatalf("FindRate: %v", err)
	}
	if got.Rate.String() != "3.3000" || got.Period != july || got.From != money.USD || got.To != money.PEN || !got.UpdatedAt.Equal(now) {
		t.Errorf("FindRate = %+v", got)
	}
	if _, err := repo.FindRate(ctx, july, money.PEN, money.USD); !errors.Is(err, exchange.ErrRateNotFound) {
		t.Errorf("reverse pair: err = %v, want %v", err, exchange.ErrRateNotFound)
	}
}

func testExchange(t *testing.T, id, incomeID string) (exchange.Exchange, income.Income) {
	rate, _ := money.ParseRate("3.25")
	inc := income.Income{
		ID:         incomeID,
		Period:     july,
		Amount:     mustMoney(t, "3562.00", money.PEN),
		Kind:       income.KindExchange,
		ReceivedOn: now,
		CreatedAt:  now,
	}
	ex := exchange.Exchange{
		ID:          id,
		Period:      july,
		From:        mustMoney(t, "1096.00", money.USD),
		To:          inc.Amount,
		Rate:        rate,
		ExchangedOn: now,
		IncomeID:    incomeID,
		CreatedAt:   now,
	}
	return ex, inc
}

func TestExchangeRepositorySave(t *testing.T) {
	db := openTestDB(t)
	repo := NewExchangeRepository(db)

	ex, inc := testExchange(t, "ex-1", "inc-1")
	if err := repo.Save(context.Background(), ex, inc); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if count(t, db, "exchanges") != 1 || count(t, db, "incomes") != 1 {
		t.Errorf("want 1 exchange and 1 income, got %d and %d", count(t, db, "exchanges"), count(t, db, "incomes"))
	}
}

func TestExchangeRepositorySaveIsAtomic(t *testing.T) {
	db := openTestDB(t)
	repo := NewExchangeRepository(db)

	first, firstIncome := testExchange(t, "ex-1", "inc-1")
	if err := repo.Save(context.Background(), first, firstIncome); err != nil {
		t.Fatalf("first Save: %v", err)
	}

	// Same exchange ID, new income: the income insert succeeds, the exchange insert
	// fails, and the transaction must roll the income back.
	dup, newIncome := testExchange(t, "ex-1", "inc-2")
	if err := repo.Save(context.Background(), dup, newIncome); err == nil {
		t.Fatal("saving a duplicated exchange ID must fail")
	}
	if got := count(t, db, "incomes"); got != 1 {
		t.Errorf("incomes = %d, want 1: the orphan income was not rolled back", got)
	}
}
