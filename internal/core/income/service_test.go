package income

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/category"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/period"
)

// fakeRepo records saved incomes and can be told to fail.
type fakeRepo struct {
	saved []Income
	err   error
}

func (f *fakeRepo) Save(_ context.Context, in Income) error {
	if f.err != nil {
		return f.err
	}
	f.saved = append(f.saved, in)
	return nil
}

func (f *fakeRepo) ListByPeriod(_ context.Context, p period.Period) ([]Income, error) {
	var out []Income
	for _, in := range f.saved {
		if in.Period == p {
			out = append(out, in)
		}
	}
	return out, nil
}

// fakeCategories: "loans" is used in every period, "unused" in none, "exchange" is a system category.
type fakeCategories struct{}

func (fakeCategories) EnsureInPeriod(_ context.Context, _ period.Period, id string) (category.Category, error) {
	switch id {
	case "loans":
		return category.Category{ID: id, Name: "Créditos", Group: category.GroupObligations}, nil
	case "exchange":
		return category.Category{ID: id, Name: "Cambio", Group: category.GroupSystem}, nil
	case "unused":
		return category.Category{}, fmt.Errorf("%w: %q", category.ErrNotInPeriod, id)
	default:
		return category.Category{}, fmt.Errorf("%w: %q", category.ErrNotFound, id)
	}
}

func newTestService(repo Repository) *Service {
	s := NewService(repo, fakeCategories{})
	s.now = func() time.Time { return time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC) }
	s.newID = func() string { return "test-id" }
	return s
}

func validInput() RegisterInput {
	return RegisterInput{
		Period:      "2026-07",
		Currency:    "USD",
		Amount:      "1330.00",
		Kind:        "salary",
		Description: "  Sueldo julio  ",
		ReceivedOn:  "2026-07-31",
	}
}

func TestRegister(t *testing.T) {
	repo := &fakeRepo{}
	svc := newTestService(repo)

	got, err := svc.Register(context.Background(), validInput())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.ID != "test-id" {
		t.Errorf("ID = %q, want %q", got.ID, "test-id")
	}
	if got.Period.String() != "2026-07" {
		t.Errorf("Period = %s, want 2026-07", got.Period)
	}
	if got.Amount.String() != "1330.00" || got.Amount.Currency() != "USD" {
		t.Errorf("Amount = %s %s, want 1330.00 USD", got.Amount, got.Amount.Currency())
	}
	if got.Kind != KindSalary {
		t.Errorf("Kind = %q, want %q", got.Kind, KindSalary)
	}
	if got.Description != "Sueldo julio" {
		t.Errorf("Description = %q, want trimmed %q", got.Description, "Sueldo julio")
	}
	if len(repo.saved) != 1 {
		t.Fatalf("saved %d incomes, want 1", len(repo.saved))
	}
}

func TestRegisterValidation(t *testing.T) {
	tests := []struct {
		name   string
		modify func(*RegisterInput)
	}{
		{"bad period", func(in *RegisterInput) { in.Period = "2026-7" }},
		{"unsupported currency", func(in *RegisterInput) { in.Currency = "$" }},
		{"bad amount", func(in *RegisterInput) { in.Amount = "abc" }},
		{"zero amount", func(in *RegisterInput) { in.Amount = "0" }},
		{"negative amount", func(in *RegisterInput) { in.Amount = "-5" }},
		{"unknown kind", func(in *RegisterInput) { in.Kind = "bonus" }},
		{"exchange kind is reserved", func(in *RegisterInput) { in.Kind = "exchange" }},
		{"long description", func(in *RegisterInput) { in.Description = strings.Repeat("a", 201) }},
		{"bad received_on", func(in *RegisterInput) { in.ReceivedOn = "31/07/2026" }},
		{"category on a salary", func(in *RegisterInput) { in.CategoryID = "loans" }},
		{"unknown category", func(in *RegisterInput) { in.Kind, in.CategoryID = "extra", "nope" }},
		{"category not used in the period", func(in *RegisterInput) { in.Kind, in.CategoryID = "extra", "unused" }},
		{"system category", func(in *RegisterInput) { in.Kind, in.CategoryID = "extra", "exchange" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepo{}
			in := validInput()
			tt.modify(&in)

			_, err := newTestService(repo).Register(context.Background(), in)
			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("err = %v, want %v", err, ErrInvalidInput)
			}
			if len(repo.saved) != 0 {
				t.Errorf("saved %d incomes, want 0", len(repo.saved))
			}
		})
	}
}

func TestRegisterRepositoryError(t *testing.T) {
	repoErr := errors.New("disk full")
	svc := newTestService(&fakeRepo{err: repoErr})

	_, err := svc.Register(context.Background(), validInput())
	if !errors.Is(err, repoErr) {
		t.Fatalf("err = %v, want %v", err, repoErr)
	}
	if errors.Is(err, ErrInvalidInput) {
		t.Error("repository error must not be reported as invalid input")
	}
}

func TestRegisterContribution(t *testing.T) {
	repo := &fakeRepo{}
	in := validInput()
	in.Kind, in.Currency, in.Amount, in.CategoryID = "extra", "PEN", "108.00", "loans"

	got, err := newTestService(repo).Register(context.Background(), in)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if got.Kind != KindExtra || got.CategoryID != "loans" {
		t.Errorf("got kind %q category %q, want extra contributed to loans", got.Kind, got.CategoryID)
	}

	list, err := newTestService(repo).List(context.Background(), "2026-07")
	if err != nil || len(list) != 1 || list[0].CategoryID != "loans" {
		t.Errorf("List = %+v, %v", list, err)
	}
	if _, err := newTestService(repo).List(context.Background(), "julio"); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("List bad period: err = %v, want %v", err, ErrInvalidInput)
	}
}
