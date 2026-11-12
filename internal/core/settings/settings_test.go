package settings

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/currency"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/money"
)

type onlyPEN struct{}

func (onlyPEN) Require(_ context.Context, c money.Currency) error {
	if c != "PEN" {
		return fmt.Errorf("%w: %s not registered", currency.ErrInvalidInput, c)
	}
	return nil
}

type fakeRepo struct{ s Settings }

func (f *fakeRepo) Get(context.Context) (Settings, error)    { return f.s, nil }
func (f *fakeRepo) Save(_ context.Context, s Settings) error { f.s = s; return nil }

func TestUpdateDefaultCurrency(t *testing.T) {
	svc := NewService(&fakeRepo{}, onlyPEN{})
	ctx := context.Background()
	pen, usd, none := "pen", "USD", ""

	got, err := svc.Update(ctx, UpdateInput{DefaultCurrency: &pen})
	if err != nil || got.DefaultCurrency != "PEN" {
		t.Fatalf("Update(pen) = %+v, %v", got, err)
	}
	if _, err := svc.Update(ctx, UpdateInput{DefaultCurrency: &usd}); !errors.Is(err, currency.ErrInvalidInput) {
		t.Errorf("unregistered default error = %v, want currency.ErrInvalidInput", err)
	}
	if got, err := svc.Update(ctx, UpdateInput{DefaultCurrency: &none}); err != nil || got.DefaultCurrency != "" {
		t.Errorf("Update(\"\") = %+v, %v, want the default cleared", got, err)
	}
}
