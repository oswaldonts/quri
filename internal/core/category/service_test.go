package category

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"
)

// fakeRepo is an in-test catalog enforcing the same rules as the real one.
type fakeRepo struct{ byID map[string]Category }

func newFakeRepo(cs ...Category) *fakeRepo {
	r := &fakeRepo{byID: map[string]Category{}}
	for _, c := range cs {
		r.byID[c.ID] = c
	}
	return r
}

func (r *fakeRepo) nameTaken(c Category) bool {
	for _, other := range r.byID {
		if other.ID != c.ID && NameKey(other.Name) == NameKey(c.Name) {
			return true
		}
	}
	return false
}

func (r *fakeRepo) Create(_ context.Context, c Category) error {
	if r.nameTaken(c) {
		return ErrNameTaken
	}
	r.byID[c.ID] = c
	return nil
}

func (r *fakeRepo) Get(_ context.Context, id string) (Category, error) {
	c, ok := r.byID[id]
	if !ok {
		return Category{}, ErrNotFound
	}
	return c, nil
}

func (r *fakeRepo) List(_ context.Context, includeArchived bool) ([]Category, error) {
	var out []Category
	for _, c := range r.byID {
		if includeArchived || !c.Archived {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (r *fakeRepo) Update(_ context.Context, c Category) error {
	if _, ok := r.byID[c.ID]; !ok {
		return ErrNotFound
	}
	if r.nameTaken(c) {
		return ErrNameTaken
	}
	r.byID[c.ID] = c
	return nil
}

var (
	market   = Category{ID: "market", Name: "Mercado", Group: GroupNeeds}
	exchange = Category{ID: "exchange", Name: "Cambio", Group: GroupSystem}
)

func newTestService(repo Repository) *Service {
	s := NewService(repo)
	s.now = func() time.Time { return time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC) }
	s.newID = func() string { return "new-id" }
	return s
}

func ptr[T any](v T) *T { return &v }

func TestCreate(t *testing.T) {
	svc := newTestService(newFakeRepo(market))

	got, err := svc.Create(context.Background(), CreateInput{Name: "  Suscripciones ", Group: "lifestyle"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.ID != "new-id" || got.Name != "Suscripciones" || got.Group != GroupLifestyle || got.Committed || got.Archived {
		t.Errorf("Create = %+v", got)
	}
}

func TestCreateErrors(t *testing.T) {
	tests := []struct {
		name    string
		in      CreateInput
		wantErr error
	}{
		{"empty name", CreateInput{Name: "  ", Group: "needs"}, ErrInvalidInput},
		{"long name", CreateInput{Name: strings.Repeat("a", 51), Group: "needs"}, ErrInvalidInput},
		{"unknown group", CreateInput{Name: "X", Group: "fun"}, ErrInvalidInput},
		{"system group is reserved", CreateInput{Name: "X", Group: "system"}, ErrInvalidInput},
		{"duplicated name, other case", CreateInput{Name: "mercado", Group: "needs"}, ErrNameTaken},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newTestService(newFakeRepo(market)).Create(context.Background(), tt.in)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestUpdatePartial(t *testing.T) {
	repo := newFakeRepo(market)
	svc := newTestService(repo)

	got, err := svc.Update(context.Background(), "market", UpdateInput{Committed: ptr(true)})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	// Only Committed changed; name and group are untouched.
	if got.Name != "Mercado" || got.Group != GroupNeeds || !got.Committed {
		t.Errorf("Update = %+v", got)
	}

	got, err = svc.Update(context.Background(), "market", UpdateInput{Name: ptr("Supermercado"), Archived: ptr(true)})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.Name != "Supermercado" || !got.Archived || !got.Committed {
		t.Errorf("Update = %+v", got)
	}
	if repo.byID["market"] != got {
		t.Error("the update was not stored")
	}
}

func TestUpdateErrors(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		in      UpdateInput
		wantErr error
	}{
		{"unknown id", "nope", UpdateInput{Name: ptr("X")}, ErrNotFound},
		{"system category", "exchange", UpdateInput{Name: ptr("Divisas")}, ErrSystemCategory},
		{"empty name", "market", UpdateInput{Name: ptr("")}, ErrInvalidInput},
		{"system group is reserved", "market", UpdateInput{Group: ptr("system")}, ErrInvalidInput},
		{"name taken", "market", UpdateInput{Name: ptr("CAMBIO")}, ErrNameTaken},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newTestService(newFakeRepo(market, exchange)).Update(context.Background(), tt.id, tt.in)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestList(t *testing.T) {
	archived := Category{ID: "old", Name: "Antiguo", Group: GroupOther, Archived: true}
	svc := newTestService(newFakeRepo(market, archived))

	active, _ := svc.List(context.Background(), false)
	all, _ := svc.List(context.Background(), true)
	if len(active) != 1 || len(all) != 2 {
		t.Errorf("active = %d, all = %d; want 1 and 2", len(active), len(all))
	}
}
