package sqlite

import (
	"context"
	"database/sql"

	"github.com/oswaldoaquinas/go-income-mgr/internal/adapter/sqlite/sqlitedb"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/settings"
)

// Compile-time check that SettingsRepository implements the core port.
var _ settings.Repository = (*SettingsRepository)(nil)

// SettingsRepository stores the installation settings in SQLite.
type SettingsRepository struct {
	q *sqlitedb.Queries
}

// NewSettingsRepository returns a repository backed by db.
func NewSettingsRepository(db *sql.DB) *SettingsRepository {
	return &SettingsRepository{q: sqlitedb.New(db)}
}

// Get returns the settings; the migration creates the row.
func (r *SettingsRepository) Get(ctx context.Context) (settings.Settings, error) {
	row, err := r.q.GetSettings(ctx)
	return settings.Settings{SpendOffset: int(row.SpendOffset), DefaultCurrency: row.DefaultCurrency.String, ClosedThrough: row.ClosedThrough.String}, err
}

// Save replaces the settings.
func (r *SettingsRepository) Save(ctx context.Context, s settings.Settings) error {
	return r.q.UpdateSettings(ctx, sqlitedb.UpdateSettingsParams{
		SpendOffset:     int64(s.SpendOffset),
		DefaultCurrency: sql.NullString{String: s.DefaultCurrency, Valid: s.DefaultCurrency != ""},
		ClosedThrough:   sql.NullString{String: s.ClosedThrough, Valid: s.ClosedThrough != ""},
	})
}
