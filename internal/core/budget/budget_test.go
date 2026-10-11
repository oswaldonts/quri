package budget

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/category"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/money"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/period"
)

type key struct {
	p   period.Period
	cat string
	c   money.Currency
}

type fakeRepo struct{ byKey map[key]Budget }

func (f *fakeRepo) Save(_ context.Context, b Budget) error {
	f.byKey[key{b.Period, b.CategoryID, b.Amount.Currency()}] = b
	return nil
}

func (f *fakeRepo) ListByPeriod(_ context.Context, p period.Period) ([]Budget, error) {
	var out []Budget
	for k, b := range f.byKey {
		if k.p == p {
			out = append(out, b)
		}
	}
	return out, nil
}

func (f *fakeRepo) Delete(_ context.Context, p period.Period, cat string, c money.Currency) error {
	if _, ok := f.byKey[key{p, cat, c}]; !ok {
		return ErrNotFound
	}
	delete(f.byKey, key{p, cat, c})
	return nil
}

type fakeCategories struct{}

func (fakeCategories) EnsureInPeriod(_ context.Context, _ period.Period, id string) (category.Category, error) {
	switch id {
	case "market":
		return category.Category{ID: id, Group: category.GroupNeeds}, nil
	case "exchange":
		return category.Category{ID: id, Name: "Cambio", Group: category.GroupSystem}, nil
	case "unused":
		return category.Category{}, fmt.Errorf("%w: %q", category.ErrNotInPeriod, id)
	default:
		return category.Category{}, fmt.Errorf("%w: %q", category.ErrNotFound, id)
	}
}

func newTestService() (*Service, *fakeRepo) {
	repo := &fakeRepo{byKey: map[key]Budget{}}
	s := NewService(repo, fakeCategories{}, anyCurrency{})
	s.now = func() time.Time { return time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC) }
	return s, repo
}

func TestSetListDelete(t *testing.T) {
	ctx := context.Background()
	svc, repo := newTestService()

	for _, amount := range []string{"300", "350"} { // the second replaces the first
		if _, err := svc.Set(ctx, SetInput{Period: "2026-08", CategoryID: "market", Currency: "PEN", Amount: amount}); err != nil {
			t.Fatalf("Set %s: %v", amount, err)
		}
	}
	list, err := svc.List(ctx, "2026-08")
	if err != nil || len(list) != 1 || list[0].Amount.String() != "350.00" {
		t.Fatalf("List = %+v, %v; want one budget of 350.00", list, err)
	}

	if err := svc.Delete(ctx, "2026-08", "market", "USD"); !errors.Is(err, ErrNotFound) {
		t.Errorf("delete missing: err = %v, want %v", err, ErrNotFound)
	}
	if err := svc.Delete(ctx, "2026-08", "market", "pen"); err != nil || len(repo.byKey) != 0 {
		t.Errorf("Delete = %v, remaining %d", err, len(repo.byKey))
	}
}

func TestSetValidation(t *testing.T) {
	tests := []struct {
		name string
		in   SetInput
	}{
		{"bad period", SetInput{Period: "2026-8", CategoryID: "market", Currency: "PEN", Amount: "1"}},
		{"unregistered currency", SetInput{Period: "2026-08", CategoryID: "market", Currency: "EUR", Amount: "1"}},
		{"bad amount", SetInput{Period: "2026-08", CategoryID: "market", Currency: "PEN", Amount: "1.005"}},
		{"zero amount", SetInput{Period: "2026-08", CategoryID: "market", Currency: "PEN", Amount: "0"}},
		{"unknown category", SetInput{Period: "2026-08", CategoryID: "nope", Currency: "PEN", Amount: "1"}},
		{"category not used", SetInput{Period: "2026-08", CategoryID: "unused", Currency: "PEN", Amount: "1"}},
		{"system category", SetInput{Period: "2026-08", CategoryID: "exchange", Currency: "PEN", Amount: "1"}},
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
	if _, err := svc.List(context.Background(), "agosto"); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("List bad period: err = %v, want %v", err, ErrInvalidInput)
	}
	if err := svc.Delete(context.Background(), "agosto", "market", "PEN"); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("Delete bad period: err = %v, want %v", err, ErrInvalidInput)
	}
	if err := svc.Delete(context.Background(), "2026-08", "market", "EU"); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("Delete bad currency: err = %v, want %v", err, ErrInvalidInput)
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
