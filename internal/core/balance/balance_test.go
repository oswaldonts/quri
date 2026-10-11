package balance

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/money"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/period"
)

type key struct {
	p period.Period
	c money.Currency
}

type fakeRepo struct{ byKey map[key]Balance }

func (f *fakeRepo) Save(_ context.Context, b Balance) error {
	f.byKey[key{b.Period, b.Amount.Currency()}] = b
	return nil
}

func (f *fakeRepo) ListByPeriod(_ context.Context, p period.Period) ([]Balance, error) {
	var out []Balance
	for k, b := range f.byKey {
		if k.p == p {
			out = append(out, b)
		}
	}
	return out, nil
}

func (f *fakeRepo) Delete(_ context.Context, p period.Period, c money.Currency) error {
	if _, ok := f.byKey[key{p, c}]; !ok {
		return ErrNotFound
	}
	delete(f.byKey, key{p, c})
	return nil
}

func newTestService() (*Service, *fakeRepo) {
	repo := &fakeRepo{byKey: map[key]Balance{}}
	s := NewService(repo, anyCurrency{})
	s.now = func() time.Time { return time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC) }
	return s, repo
}

func TestSetReplaces(t *testing.T) {
	ctx := context.Background()
	svc, repo := newTestService()

	for _, amount := range []string{"400.00", "415.50"} {
		if _, err := svc.Set(ctx, SetInput{Period: "2026-07", Currency: "PEN", Amount: amount}); err != nil {
			t.Fatalf("Set %s: %v", amount, err)
		}
	}
	list, err := svc.List(ctx, "2026-07")
	if err != nil || len(list) != 1 || list[0].Amount.String() != "415.50" {
		t.Errorf("List = %+v, %v; want one PEN balance of 415.50 (last one wins)", list, err)
	}
	if len(repo.byKey) != 1 {
		t.Errorf("stored %d balances, want 1", len(repo.byKey))
	}
}

func TestSetAcceptsZeroAndNegative(t *testing.T) {
	svc, _ := newTestService()
	for _, amount := range []string{"0", "-20.00"} {
		if _, err := svc.Set(context.Background(), SetInput{Period: "2026-07", Currency: "USD", Amount: amount}); err != nil {
			t.Errorf("Set %s: %v", amount, err)
		}
	}
}

func TestSetValidation(t *testing.T) {
	tests := []struct {
		name string
		in   SetInput
	}{
		{"bad period", SetInput{Period: "2026-7", Currency: "PEN", Amount: "1"}},
		{"unregistered currency", SetInput{Period: "2026-07", Currency: "EUR", Amount: "1"}},
		{"bad amount", SetInput{Period: "2026-07", Currency: "PEN", Amount: "1.005"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _ := newTestService()
			if _, err := svc.Set(context.Background(), tt.in); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("err = %v, want %v", err, ErrInvalidInput)
			}
		})
	}
	svc, _ := newTestService()
	if _, err := svc.List(context.Background(), "julio"); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("List bad period: err = %v, want %v", err, ErrInvalidInput)
	}
}

func TestDelete(t *testing.T) {
	ctx := context.Background()
	svc, repo := newTestService()
	_, _ = svc.Set(ctx, SetInput{Period: "2026-07", Currency: "PEN", Amount: "10"})

	if err := svc.Delete(ctx, "2026-07", "USD"); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing balance: err = %v, want %v", err, ErrNotFound)
	}
	if err := svc.Delete(ctx, "2026-07", "EU"); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("bad currency: err = %v, want %v", err, ErrInvalidInput)
	}
	if err := svc.Delete(ctx, "julio", "PEN"); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("bad period: err = %v, want %v", err, ErrInvalidInput)
	}
	if err := svc.Delete(ctx, "2026-07", "pen"); err != nil || len(repo.byKey) != 0 {
		t.Errorf("Delete = %v, remaining %d", err, len(repo.byKey))
	}
}

// anyCurrency accepts every currency except EUR, which tests use as the unregistered one.
type anyCurrency struct{}

func (anyCurrency) Require(_ context.Context, c money.Currency) error {
	if c == "EUR" {
		return ErrInvalidInput // stands for a currency that was never registered
	}
	return nil
}
