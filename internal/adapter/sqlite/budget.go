package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/oswaldoaquinas/go-income-mgr/internal/adapter/sqlite/sqlitedb"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/budget"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/money"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/period"
)

// Compile-time check that BudgetRepository implements the core port.
var _ budget.Repository = (*BudgetRepository)(nil)

// BudgetRepository stores budgets in SQLite.
type BudgetRepository struct {
	q *sqlitedb.Queries
}

// NewBudgetRepository returns a repository backed by db.
func NewBudgetRepository(db *sql.DB) *BudgetRepository {
	return &BudgetRepository{q: sqlitedb.New(db)}
}

// Save inserts the budget or replaces the existing one.
func (r *BudgetRepository) Save(ctx context.Context, b budget.Budget) error {
	return r.q.UpsertBudget(ctx, sqlitedb.UpsertBudgetParams{
		Period:      b.Period.String(),
		CategoryID:  b.CategoryID,
		Currency:    string(b.Amount.Currency()),
		AmountMinor: b.Amount.Amount(),
		UpdatedAt:   formatTime(b.UpdatedAt),
	})
}

// ListByPeriod returns the budgets of the period.
func (r *BudgetRepository) ListByPeriod(ctx context.Context, p period.Period) ([]budget.Budget, error) {
	rows, err := r.q.ListBudgetsByPeriod(ctx, p.String())
	if err != nil {
		return nil, err
	}
	out := make([]budget.Budget, 0, len(rows))
	for _, row := range rows {
		currency, err := money.ParseCurrency(row.Currency)
		if err != nil {
			return nil, fmt.Errorf("budget row: %w", err)
		}
		amount, err := money.New(row.AmountMinor, currency)
		if err != nil {
			return nil, fmt.Errorf("budget row: %w", err)
		}
		updatedAt, err := parseTime(row.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("budget row: %w", err)
		}
		out = append(out, budget.Budget{Period: p, CategoryID: row.CategoryID, Amount: amount, UpdatedAt: updatedAt})
	}
	return out, nil
}

// Delete removes a budget.
func (r *BudgetRepository) Delete(ctx context.Context, p period.Period, categoryID string, c money.Currency) error {
	n, err := r.q.DeleteBudget(ctx, sqlitedb.DeleteBudgetParams{Period: p.String(), CategoryID: categoryID, Currency: string(c)})
	return notFoundIfNone(n, err, budget.ErrNotFound, fmt.Sprintf("%s %s %s", p, categoryID, c))
}
