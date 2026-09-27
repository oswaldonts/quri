package expense

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/category"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/period"
)

type fakeRepo struct{ saved []Expense }

func (f *fakeRepo) Save(_ context.Context, e Expense) error {
	f.saved = append(f.saved, e)
	return nil
}

func (f *fakeRepo) ListByPeriod(_ context.Context, p period.Period) ([]Expense, error) {
	var out []Expense
	for _, e := range f.saved {
		if e.Period == p {
			out = append(out, e)
		}
	}
	return out, nil
}

// fakeCategories knows which categories each period uses.
type fakeCategories struct{ inPeriod map[string]category.Category }

func (f fakeCategories) EnsureInPeriod(_ context.Context, _ period.Period, id string) (category.Category, error) {
	if id == "unused" {
		return category.Category{}, fmt.Errorf("%w: %q", category.ErrNotInPeriod, id)
	}
	c, ok := f.inPeriod[id]
	if !ok {
		return category.Category{}, fmt.Errorf("%w: %q", category.ErrNotFound, id)
	}
	return c, nil
}

func newTestService(repo *fakeRepo) *Service {
	s := NewService(repo, fakeCategories{inPeriod: map[string]category.Category{
		"market":   {ID: "market", Name: "Mercado", Group: category.GroupNeeds},
		"exchange": {ID: "exchange", Name: "Cambio", Group: category.GroupSystem},
	}})
	s.now = func() time.Time { return time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC) }
	s.newID = func() string { return "exp-1" }
	return s
}

func validInput() RegisterInput {
	return RegisterInput{
		Period:      "2026-07",
		CategoryID:  "market",
		Currency:    "PEN",
		Amount:      "45.50",
		Description: " Mercado semanal ",
		SpentOn:     "2026-08-03",
	}
}

func TestRegister(t *testing.T) {
	repo := &fakeRepo{}
	got, err := newTestService(repo).Register(context.Background(), validInput())
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if got.ID != "exp-1" || got.CategoryID != "market" || got.Amount.String() != "45.50" ||
		got.Amount.Currency() != "PEN" || got.Description != "Mercado semanal" || got.ExchangeID != "" {
		t.Errorf("Register = %+v", got)
	}
	if len(repo.saved) != 1 {
		t.Fatalf("saved %d expenses, want 1", len(repo.saved))
	}

	list, err := newTestService(repo).List(context.Background(), "2026-07")
	if err != nil || len(list) != 1 {
		t.Errorf("List = %d expenses, %v; want 1", len(list), err)
	}
}

func TestRegisterValidation(t *testing.T) {
	tests := []struct {
		name   string
		modify func(*RegisterInput)
	}{
		{"bad period", func(in *RegisterInput) { in.Period = "2026-7" }},
		{"unsupported currency", func(in *RegisterInput) { in.Currency = "EUR" }},
		{"bad amount", func(in *RegisterInput) { in.Amount = "1.005" }},
		{"zero amount", func(in *RegisterInput) { in.Amount = "0" }},
		{"long description", func(in *RegisterInput) { in.Description = strings.Repeat("a", 201) }},
		{"bad date", func(in *RegisterInput) { in.SpentOn = "03/08/2026" }},
		{"unknown category", func(in *RegisterInput) { in.CategoryID = "nope" }},
		{"category not used in the period", func(in *RegisterInput) { in.CategoryID = "unused" }},
		{"system category", func(in *RegisterInput) { in.CategoryID = "exchange" }},
		{"empty tag", func(in *RegisterInput) { in.Tags = []string{"dulces", "  "} }},
		{"long tag", func(in *RegisterInput) { in.Tags = []string{strings.Repeat("a", 31)} }},
		{"too many tags", func(in *RegisterInput) {
			in.Tags = []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"}
		}},
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
				t.Error("nothing must be saved on invalid input")
			}
		})
	}
}

func TestListBadPeriod(t *testing.T) {
	if _, err := newTestService(&fakeRepo{}).List(context.Background(), "julio"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("err = %v, want %v", err, ErrInvalidInput)
	}
}

func TestNormalizeTags(t *testing.T) {
	got, err := NormalizeTags([]string{"#Dulces", " dulces ", "Frutos   Secos", "snacks"})
	if err != nil {
		t.Fatalf("NormalizeTags: %v", err)
	}
	want := []string{"dulces", "frutos secos", "snacks"}
	if !slices.Equal(got, want) {
		t.Errorf("NormalizeTags = %q, want %q", got, want)
	}
	if got, _ := NormalizeTags(nil); len(got) != 0 {
		t.Errorf("NormalizeTags(nil) = %q, want empty", got)
	}
}

func TestRegisterWithTags(t *testing.T) {
	in := validInput()
	in.Tags = []string{"Lácteos", "#desayuno"}
	got, err := newTestService(&fakeRepo{}).Register(context.Background(), in)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if want := []string{"desayuno", "lácteos"}; !slices.Equal(got.Tags, want) {
		t.Errorf("Tags = %q, want %q", got.Tags, want)
	}
}
