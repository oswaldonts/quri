package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/oswaldoaquinas/go-income-mgr/internal/adapter/sqlite/sqlitedb"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/balance"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/money"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/period"
)

// Compile-time check that BalanceRepository implements the core port.
var _ balance.Repository = (*BalanceRepository)(nil)

// BalanceRepository stores real balances in SQLite.
type BalanceRepository struct {
	q *sqlitedb.Queries
}

// NewBalanceRepository returns a repository backed by db.
func NewBalanceRepository(db *sql.DB) *BalanceRepository {
	return &BalanceRepository{q: sqlitedb.New(db)}
}

// Save inserts the balance or replaces the existing one for its period and currency.
func (r *BalanceRepository) Save(ctx context.Context, b balance.Balance) error {
	return r.q.UpsertBalance(ctx, sqlitedb.UpsertBalanceParams{
		Period:      b.Period.String(),
		Currency:    string(b.Amount.Currency()),
		AmountMinor: b.Amount.Amount(),
		UpdatedAt:   formatTime(b.UpdatedAt),
	})
}

// ListByPeriod returns the balances of the period ordered by currency.
func (r *BalanceRepository) ListByPeriod(ctx context.Context, p period.Period) ([]balance.Balance, error) {
	rows, err := r.q.ListBalancesByPeriod(ctx, p.String())
	if err != nil {
		return nil, err
	}
	out := make([]balance.Balance, 0, len(rows))
	for _, row := range rows {
		currency, err := money.ParseCurrency(row.Currency)
		if err != nil {
			return nil, fmt.Errorf("balance row: %w", err)
		}
		amount, err := money.New(row.AmountMinor, currency)
		if err != nil {
			return nil, fmt.Errorf("balance row: %w", err)
		}
		updatedAt, err := parseTime(row.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("balance row: %w", err)
		}
		out = append(out, balance.Balance{Period: p, Amount: amount, UpdatedAt: updatedAt})
	}
	return out, nil
}

// Delete removes the balance of a currency in a period.
func (r *BalanceRepository) Delete(ctx context.Context, p period.Period, c money.Currency) error {
	n, err := r.q.DeleteBalance(ctx, sqlitedb.DeleteBalanceParams{Period: p.String(), Currency: string(c)})
	return notFoundIfNone(n, err, balance.ErrNotFound, fmt.Sprintf("%s %s", p, c))
}
