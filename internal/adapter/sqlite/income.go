package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/oswaldoaquinas/go-income-mgr/internal/adapter/sqlite/sqlitedb"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/income"
)

// Compile-time check that IncomeRepository implements the core port.
var _ income.Repository = (*IncomeRepository)(nil)

// IncomeRepository stores incomes in SQLite.
type IncomeRepository struct {
	q *sqlitedb.Queries
}

// NewIncomeRepository returns a repository backed by db.
func NewIncomeRepository(db *sql.DB) *IncomeRepository {
	return &IncomeRepository{q: sqlitedb.New(db)}
}

// Save inserts the income.
func (r *IncomeRepository) Save(ctx context.Context, in income.Income) error {
	return insertIncome(ctx, r.q, in)
}

// insertIncome is shared with ExchangeRepository, which inserts inside a transaction.
func insertIncome(ctx context.Context, q *sqlitedb.Queries, in income.Income) error {
	return q.InsertIncome(ctx, sqlitedb.InsertIncomeParams{
		ID:          in.ID,
		Period:      in.Period.String(),
		Currency:    string(in.Amount.Currency()),
		AmountMinor: in.Amount.Amount(),
		Kind:        string(in.Kind),
		Description: in.Description,
		ReceivedOn:  in.ReceivedOn.Format(time.DateOnly),
		CreatedAt:   formatTime(in.CreatedAt),
	})
}
