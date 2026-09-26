package memory

import (
	"context"
	"errors"
	"testing"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/exchange"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/money"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/period"
)

func TestRateRepository(t *testing.T) {
	ctx := context.Background()
	repo := NewRateRepository()
	july, _ := period.Parse("2026-07")
	august, _ := period.Parse("2026-08")

	if _, err := repo.FindRate(ctx, july, money.USD, money.PEN); !errors.Is(err, exchange.ErrRateNotFound) {
		t.Fatalf("empty repo: err = %v, want %v", err, exchange.ErrRateNotFound)
	}

	first, _ := money.ParseRate("3.25")
	second, _ := money.ParseRate("3.30")
	_ = repo.SaveRate(ctx, exchange.PeriodRate{Period: july, From: money.USD, To: money.PEN, Rate: first})
	_ = repo.SaveRate(ctx, exchange.PeriodRate{Period: july, From: money.USD, To: money.PEN, Rate: second})

	got, err := repo.FindRate(ctx, july, money.USD, money.PEN)
	if err != nil {
		t.Fatalf("FindRate: %v", err)
	}
	if got.Rate != second {
		t.Errorf("rate = %s, want %s (last saved wins)", got.Rate, second)
	}

	// Other periods and the reverse pair are independent.
	if _, err := repo.FindRate(ctx, august, money.USD, money.PEN); !errors.Is(err, exchange.ErrRateNotFound) {
		t.Errorf("other period: err = %v, want %v", err, exchange.ErrRateNotFound)
	}
	if _, err := repo.FindRate(ctx, july, money.PEN, money.USD); !errors.Is(err, exchange.ErrRateNotFound) {
		t.Errorf("reverse pair: err = %v, want %v", err, exchange.ErrRateNotFound)
	}
}

func TestExchangeRepositorySave(t *testing.T) {
	repo := NewExchangeRepository()
	if err := repo.Save(context.Background(), exchange.Exchange{ID: "x"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if len(repo.exchanges) != 1 {
		t.Errorf("stored %d exchanges, want 1", len(repo.exchanges))
	}
}
