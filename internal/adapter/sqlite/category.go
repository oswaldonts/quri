package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"github.com/oswaldoaquinas/go-income-mgr/internal/adapter/sqlite/sqlitedb"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/category"
)

// Compile-time check that CategoryRepository implements the core port.
var _ category.Repository = (*CategoryRepository)(nil)

// CategoryRepository stores the category catalog in SQLite.
type CategoryRepository struct {
	q   *sqlitedb.Queries
	now func() time.Time // timestamp recorded when a category gets archived
}

// NewCategoryRepository returns a repository backed by db.
func NewCategoryRepository(db *sql.DB) *CategoryRepository {
	return &CategoryRepository{q: sqlitedb.New(db), now: time.Now}
}

// Create inserts the category; a duplicated name returns category.ErrNameTaken.
func (r *CategoryRepository) Create(ctx context.Context, c category.Category) error {
	var archivedAt sql.NullString
	if c.Archived {
		archivedAt = sql.NullString{String: formatTime(r.now()), Valid: true}
	}
	err := r.q.InsertCategory(ctx, sqlitedb.InsertCategoryParams{
		ID:         c.ID,
		Name:       c.Name,
		NameKey:    category.NameKey(c.Name),
		GroupKey:   string(c.Group),
		Committed:  boolToInt(c.Committed),
		ArchivedAt: archivedAt,
		CreatedAt:  formatTime(c.CreatedAt),
	})
	return mapCategoryError(err)
}

// Get returns category.ErrNotFound if no category has the ID.
func (r *CategoryRepository) Get(ctx context.Context, id string) (category.Category, error) {
	row, err := r.q.GetCategory(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return category.Category{}, fmt.Errorf("%w: %q", category.ErrNotFound, id)
	}
	if err != nil {
		return category.Category{}, err
	}
	return toCategory(row)
}

// List returns the catalog ordered by group and name.
func (r *CategoryRepository) List(ctx context.Context, includeArchived bool) ([]category.Category, error) {
	rows, err := r.q.ListCategories(ctx, includeArchived)
	if err != nil {
		return nil, err
	}
	out := make([]category.Category, 0, len(rows))
	for _, row := range rows {
		c, err := toCategory(row)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

// Update replaces name, group, committed and archived state of an existing category.
func (r *CategoryRepository) Update(ctx context.Context, c category.Category) error {
	n, err := r.q.UpdateCategory(ctx, sqlitedb.UpdateCategoryParams{
		Name:      c.Name,
		NameKey:   category.NameKey(c.Name),
		GroupKey:  string(c.Group),
		Committed: boolToInt(c.Committed),
		Archived:  c.Archived,
		Now:       sql.NullString{String: formatTime(r.now()), Valid: true},
		ID:        c.ID,
	})
	if err != nil {
		return mapCategoryError(err)
	}
	if n == 0 {
		return fmt.Errorf("%w: %q", category.ErrNotFound, c.ID)
	}
	return nil
}

func toCategory(row sqlitedb.Category) (category.Category, error) {
	createdAt, err := parseTime(row.CreatedAt)
	if err != nil {
		return category.Category{}, fmt.Errorf("category row: %w", err)
	}
	return category.Category{
		ID:        row.ID,
		Name:      row.Name,
		Group:     category.Group(row.GroupKey),
		Committed: row.Committed == 1,
		Archived:  row.ArchivedAt.Valid,
		CreatedAt: createdAt,
	}, nil
}

// mapCategoryError turns the UNIQUE violation on name_key into category.ErrNameTaken.
func mapCategoryError(err error) error {
	var sqlErr *sqlite.Error
	if errors.As(err, &sqlErr) && sqlErr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE {
		return category.ErrNameTaken
	}
	return err
}

// SQLite has no boolean type: booleans are stored as 0/1.
func boolToInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
