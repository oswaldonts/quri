package sqlite

import (
	"context"
	"errors"
	"testing"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/category"
)

func TestCategorySeed(t *testing.T) {
	repo := NewCategoryRepository(openTestDB(t))

	all, err := repo.List(context.Background(), false)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 18 {
		t.Fatalf("seeded %d categories, want 18", len(all))
	}
	// Ordered by group (needs first, system last) and by name within the group.
	if first, last := all[0], all[len(all)-1]; first.Name != "Cuidado personal" || last.ID != "exchange" {
		t.Errorf("first = %q, last = %q; want Cuidado personal … Cambio", first.Name, last.Name)
	}

	utilities, err := repo.Get(context.Background(), "utilities")
	if err != nil || utilities.Name != "Servicios" || !utilities.Committed || utilities.Group != category.GroupNeeds {
		t.Errorf("Get(utilities) = %+v, %v", utilities, err)
	}
}

func TestCategoryCreateAndUpdate(t *testing.T) {
	ctx := context.Background()
	repo := NewCategoryRepository(openTestDB(t))

	pets := category.Category{ID: "pets", Name: "Mascotas", Group: category.GroupNeeds, CreatedAt: now}
	if err := repo.Create(ctx, pets); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := repo.Create(ctx, category.Category{ID: "x", Name: " mercado ", Group: category.GroupNeeds, CreatedAt: now}); !errors.Is(err, category.ErrNameTaken) {
		t.Errorf("duplicated name: err = %v, want %v", err, category.ErrNameTaken)
	}

	pets.Name, pets.Committed, pets.Archived = "Mascota", true, true
	if err := repo.Update(ctx, pets); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ := repo.Get(ctx, "pets")
	if got.Name != "Mascota" || !got.Committed || !got.Archived {
		t.Errorf("after Update = %+v", got)
	}

	active, _ := repo.List(ctx, false)
	all, _ := repo.List(ctx, true)
	if len(all)-len(active) != 1 {
		t.Errorf("archived categories must be listed only with includeArchived")
	}

	pets.Name = "Hogar"
	if err := repo.Update(ctx, pets); !errors.Is(err, category.ErrNameTaken) {
		t.Errorf("rename to taken name: err = %v, want %v", err, category.ErrNameTaken)
	}
	if err := repo.Update(ctx, category.Category{ID: "nope", Name: "Nada", Group: category.GroupOther}); !errors.Is(err, category.ErrNotFound) {
		t.Errorf("unknown id: err = %v, want %v", err, category.ErrNotFound)
	}
	if _, err := repo.Get(ctx, "nope"); !errors.Is(err, category.ErrNotFound) {
		t.Errorf("Get unknown: err = %v, want %v", err, category.ErrNotFound)
	}
}

func TestCategoryArchiveKeepsTimestamp(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	repo := NewCategoryRepository(db)
	archivedAt := func() string {
		var s string
		if err := db.QueryRow("SELECT archived_at FROM categories WHERE id = 'other'").Scan(&s); err != nil {
			t.Fatalf("read archived_at: %v", err)
		}
		return s
	}

	other, _ := repo.Get(ctx, "other")
	other.Archived = true
	_ = repo.Update(ctx, other)
	first := archivedAt()

	other.Committed = true // another update while archived must not move the timestamp
	_ = repo.Update(ctx, other)
	if got := archivedAt(); got != first {
		t.Errorf("archived_at changed from %s to %s", first, got)
	}
}
