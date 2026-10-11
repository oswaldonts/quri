package currency

import (
	"context"
	"errors"
	"testing"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/money"
)

type fakeRepo struct {
	items map[money.Currency]Currency
	used  map[money.Currency]bool
}

func (f *fakeRepo) Save(_ context.Context, c Currency) error {
	if _, ok := f.items[c.Code]; ok {
		return ErrExists
	}
	f.items[c.Code] = c
	return nil
}

func (f *fakeRepo) Get(_ context.Context, code money.Currency) (Currency, error) {
	c, ok := f.items[code]
	if !ok {
		return Currency{}, ErrNotFound
	}
	return c, nil
}

func (f *fakeRepo) Update(_ context.Context, c Currency) error {
	if _, ok := f.items[c.Code]; !ok {
		return ErrNotFound
	}
	f.items[c.Code] = c
	return nil
}

func (f *fakeRepo) Delete(_ context.Context, code money.Currency) error {
	delete(f.items, code)
	return nil
}

func (f *fakeRepo) List(context.Context) ([]Currency, error) { return nil, nil }

func (f *fakeRepo) InUse(_ context.Context, code money.Currency) (bool, error) {
	return f.used[code], nil
}

func newService() (*Service, *fakeRepo) {
	repo := &fakeRepo{items: map[money.Currency]Currency{}, used: map[money.Currency]bool{}}
	return NewService(repo), repo
}

func TestCreateNormalizesAndRejects(t *testing.T) {
	s, _ := newService()
	ctx := context.Background()

	c, err := s.Create(ctx, CreateInput{Code: " eur ", Symbol: " € "})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if c.Code != "EUR" || c.Symbol != "€" {
		t.Errorf("Create = %+v, want EUR €", c)
	}
	if _, err := s.Create(ctx, CreateInput{Code: "EUR"}); !errors.Is(err, ErrExists) {
		t.Errorf("repeated code: error = %v, want ErrExists", err)
	}
	for _, in := range []CreateInput{{Code: "EURO"}, {Code: "E1R"}, {Code: "GBP", Symbol: "£££££"}} {
		if _, err := s.Create(ctx, in); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("Create(%+v): error = %v, want ErrInvalidInput", in, err)
		}
	}
}

func TestRequire(t *testing.T) {
	s, _ := newService()
	ctx := context.Background()
	if err := s.Require(ctx, "EUR"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("unregistered: error = %v, want ErrInvalidInput", err)
	}
	if _, err := s.Create(ctx, CreateInput{Code: "EUR"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Require(ctx, "EUR"); err != nil {
		t.Errorf("registered: error = %v", err)
	}
}

func TestDeleteRefusesCurrenciesInUse(t *testing.T) {
	s, repo := newService()
	ctx := context.Background()
	if _, err := s.Create(ctx, CreateInput{Code: "EUR"}); err != nil {
		t.Fatal(err)
	}
	repo.used["EUR"] = true
	if err := s.Delete(ctx, "EUR"); !errors.Is(err, ErrInUse) {
		t.Fatalf("in use: error = %v, want ErrInUse", err)
	}
	repo.used["EUR"] = false
	if err := s.Delete(ctx, "EUR"); err != nil {
		t.Fatalf("unused: error = %v", err)
	}
	if err := s.Delete(ctx, "EUR"); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing: error = %v, want ErrNotFound", err)
	}
}
