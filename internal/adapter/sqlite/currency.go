package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"github.com/oswaldoaquinas/go-income-mgr/internal/adapter/sqlite/sqlitedb"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/currency"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/money"
)

// Compile-time check that CurrencyRepository implements the core port.
var _ currency.Repository = (*CurrencyRepository)(nil)

// CurrencyRepository stores the registered currencies in SQLite.
type CurrencyRepository struct {
	q *sqlitedb.Queries
}

// NewCurrencyRepository returns a repository backed by db.
func NewCurrencyRepository(db *sql.DB) *CurrencyRepository {
	return &CurrencyRepository{q: sqlitedb.New(db)}
}

// Save inserts the currency; a repeated code is currency.ErrExists.
func (r *CurrencyRepository) Save(ctx context.Context, c currency.Currency) error {
	err := r.q.InsertCurrency(ctx, sqlitedb.InsertCurrencyParams{Code: string(c.Code), Symbol: c.Symbol})
	var sqlErr *sqlite.Error
	if errors.As(err, &sqlErr) && (sqlErr.Code() == sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY || sqlErr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE) {
		return fmt.Errorf("%w: %s", currency.ErrExists, c.Code)
	}
	return err
}

// Get returns currency.ErrNotFound if the code is not registered.
func (r *CurrencyRepository) Get(ctx context.Context, code money.Currency) (currency.Currency, error) {
	row, err := r.q.GetCurrency(ctx, string(code))
	if errors.Is(err, sql.ErrNoRows) {
		return currency.Currency{}, fmt.Errorf("%w: %s", currency.ErrNotFound, code)
	}
	if err != nil {
		return currency.Currency{}, err
	}
	return currency.Currency{Code: money.Currency(row.Code), Symbol: row.Symbol}, nil
}

// Update replaces the symbol of a currency.
func (r *CurrencyRepository) Update(ctx context.Context, c currency.Currency) error {
	n, err := r.q.UpdateCurrency(ctx, sqlitedb.UpdateCurrencyParams{Symbol: c.Symbol, Code: string(c.Code)})
	return notFoundIfNone(n, err, currency.ErrNotFound, string(c.Code))
}

// Delete removes a currency.
func (r *CurrencyRepository) Delete(ctx context.Context, code money.Currency) error {
	n, err := r.q.DeleteCurrency(ctx, string(code))
	return notFoundIfNone(n, err, currency.ErrNotFound, string(code))
}

// List returns every currency ordered by code.
func (r *CurrencyRepository) List(ctx context.Context) ([]currency.Currency, error) {
	rows, err := r.q.ListCurrencies(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]currency.Currency, 0, len(rows))
	for _, row := range rows {
		out = append(out, currency.Currency{Code: money.Currency(row.Code), Symbol: row.Symbol})
	}
	return out, nil
}

// InUse tells whether any stored amount is in the currency.
func (r *CurrencyRepository) InUse(ctx context.Context, code money.Currency) (bool, error) {
	in, err := r.q.CurrencyInUse(ctx, string(code))
	return in.Bool, err
}
