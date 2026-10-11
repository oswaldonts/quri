package income

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/category"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/money"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/period"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/status"
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

func (f *fakeRepo) Get(_ context.Context, id string) (Income, error) {
	for _, in := range f.saved {
		if in.ID == id {
			return in, nil
		}
	}
	return Income{}, ErrNotFound
}

func (f *fakeRepo) Update(_ context.Context, in Income) error {
	for i := range f.saved {
		if f.saved[i].ID == in.ID {
			f.saved[i] = in
			return nil
		}
	}
	return ErrNotFound
}

func (f *fakeRepo) Delete(_ context.Context, id string) error {
	for i := range f.saved {
		if f.saved[i].ID == id {
			f.saved = append(f.saved[:i], f.saved[i+1:]...)
			return nil
		}
	}
	return ErrNotFound
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
	s := NewService(repo, fakeCategories{}, anyCurrency{})
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

	list, err := newTestService(repo).List(context.Background(), "2026-07", "")
	if err != nil || len(list) != 1 || list[0].CategoryID != "loans" {
		t.Errorf("List = %+v, %v", list, err)
	}
	if _, err := newTestService(repo).List(context.Background(), "julio", ""); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("List bad period: err = %v, want %v", err, ErrInvalidInput)
	}
}

func ptr[T any](v T) *T { return &v }

func TestUpdate(t *testing.T) {
	ctx := context.Background()
	repo := &fakeRepo{}
	svc := newTestService(repo)
	created, err := svc.Register(ctx, validInput())
	if err != nil {
		t.Fatal(err)
	}

	got, err := svc.Update(ctx, "2026-07", created.ID, UpdateInput{Amount: ptr("1350.50"), Description: ptr("Sueldo corregido")})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.Amount.String() != "1350.50" || got.Description != "Sueldo corregido" || got.Kind != KindSalary ||
		got.ID != created.ID || !got.CreatedAt.Equal(created.CreatedAt) {
		t.Errorf("Update = %+v", got)
	}
	if repo.saved[0].Amount.String() != "1350.50" {
		t.Error("the update was not stored")
	}

	// Turning it into a contribution: kind and category change together.
	got, err = svc.Update(ctx, "2026-07", created.ID, UpdateInput{Kind: ptr("extra"), CategoryID: ptr("loans")})
	if err != nil || got.CategoryID != "loans" {
		t.Fatalf("Update to contribution = %+v, %v", got, err)
	}
	// Back to salary while keeping the category is invalid; clearing it works.
	if _, err := svc.Update(ctx, "2026-07", created.ID, UpdateInput{Kind: ptr("salary")}); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("salary with category: err = %v, want %v", err, ErrInvalidInput)
	}
	if got, err := svc.Update(ctx, "2026-07", created.ID, UpdateInput{Kind: ptr("salary"), CategoryID: ptr("")}); err != nil || got.CategoryID != "" {
		t.Errorf("salary without category = %+v, %v", got, err)
	}
}

func TestUpdateAndDeleteErrors(t *testing.T) {
	ctx := context.Background()
	repo := &fakeRepo{saved: []Income{
		{ID: "salary", Period: mustPeriod(t, "2026-07"), Kind: KindSalary},
		{ID: "from-exchange", Period: mustPeriod(t, "2026-07"), Kind: KindExchange},
	}}
	svc := newTestService(repo)

	if _, err := svc.Update(ctx, "2026-07", "nope", UpdateInput{}); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown id: err = %v, want %v", err, ErrNotFound)
	}
	if _, err := svc.Update(ctx, "2026-08", "salary", UpdateInput{}); !errors.Is(err, ErrNotFound) {
		t.Errorf("other period: err = %v, want %v", err, ErrNotFound)
	}
	if _, err := svc.Update(ctx, "2026-07", "from-exchange", UpdateInput{}); !errors.Is(err, ErrManagedByExchange) {
		t.Errorf("exchange income: err = %v, want %v", err, ErrManagedByExchange)
	}
	if err := svc.Delete(ctx, "2026-07", "from-exchange"); !errors.Is(err, ErrManagedByExchange) {
		t.Errorf("delete exchange income: err = %v, want %v", err, ErrManagedByExchange)
	}
	if err := svc.Delete(ctx, "julio", "salary"); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("bad period: err = %v, want %v", err, ErrInvalidInput)
	}
	if err := svc.Delete(ctx, "2026-07", "salary"); err != nil || len(repo.saved) != 1 {
		t.Errorf("Delete = %v, remaining %d; want nil and 1", err, len(repo.saved))
	}
}

func mustPeriod(t *testing.T, s string) period.Period {
	t.Helper()
	p, err := period.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestStatus(t *testing.T) {
	ctx := context.Background()
	repo := &fakeRepo{}
	svc := newTestService(repo)

	in := validInput()
	in.Status = "planned"
	planned, err := svc.Register(ctx, in)
	if err != nil || planned.Status != status.Planned {
		t.Fatalf("Register planned = %q, %v", planned.Status, err)
	}
	// Confirming can also correct the amount.
	got, err := svc.Update(ctx, "2026-07", planned.ID, UpdateInput{Status: ptr("confirmed"), Amount: ptr("1340.00")})
	if err != nil || got.Status != status.Confirmed || got.Amount.String() != "1340.00" {
		t.Errorf("confirm = %+v, %v", got, err)
	}

	// Default is confirmed; an unknown status is invalid.
	if def, _ := svc.Register(ctx, validInput()); def.Status != status.Confirmed {
		t.Errorf("default status = %q, want confirmed", def.Status)
	}
	bad := validInput()
	bad.Status = "pending"
	if _, err := svc.Register(ctx, bad); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("unknown status: err = %v, want %v", err, ErrInvalidInput)
	}
}

func TestListStatusFilter(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(&fakeRepo{})
	planned := validInput()
	planned.Status = "planned"
	_, _ = svc.Register(ctx, planned)
	_, _ = svc.Register(ctx, validInput())

	for filter, want := range map[string]int{"": 2, "planned": 1, "confirmed": 1} {
		if got, err := svc.List(ctx, "2026-07", filter); err != nil || len(got) != want {
			t.Errorf("List(%q) = %d, %v; want %d", filter, len(got), err, want)
		}
	}
	if _, err := svc.List(ctx, "2026-07", "pending"); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("bad filter: err = %v, want %v", err, ErrInvalidInput)
	}
}

// anyCurrency accepts every currency.
type anyCurrency struct{}

func (anyCurrency) Require(context.Context, money.Currency) error { return nil }
