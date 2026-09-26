package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/oswaldoaquinas/go-income-mgr/internal/adapter/sqlite/sqlitedb"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/expense"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/money"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/period"
)

// Compile-time check that ExpenseRepository implements the core port.
var _ expense.Repository = (*ExpenseRepository)(nil)

// ExpenseRepository stores expenses in SQLite.
type ExpenseRepository struct {
	q *sqlitedb.Queries
}

// NewExpenseRepository returns a repository backed by db.
func NewExpenseRepository(db *sql.DB) *ExpenseRepository {
	return &ExpenseRepository{q: sqlitedb.New(db)}
}

// Save inserts the expense.
func (r *ExpenseRepository) Save(ctx context.Context, e expense.Expense) error {
	return insertExpense(ctx, r.q, e)
}

// ListByPeriod returns the expenses of the period ordered by date.
func (r *ExpenseRepository) ListByPeriod(ctx context.Context, p period.Period) ([]expense.Expense, error) {
	rows, err := r.q.ListExpensesByPeriod(ctx, p.String())
	if err != nil {
		return nil, err
	}
	out := make([]expense.Expense, 0, len(rows))
	for _, row := range rows {
		e, err := toExpense(row)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

// insertExpense is shared with ExchangeRepository, which inserts inside a transaction.
func insertExpense(ctx context.Context, q *sqlitedb.Queries, e expense.Expense) error {
	return q.InsertExpense(ctx, sqlitedb.InsertExpenseParams{
		ID:          e.ID,
		Period:      e.Period.String(),
		CategoryID:  e.CategoryID,
		Currency:    string(e.Amount.Currency()),
		AmountMinor: e.Amount.Amount(),
		Description: e.Description,
		SpentOn:     e.SpentOn.Format(time.DateOnly),
		ExchangeID:  sql.NullString{String: e.ExchangeID, Valid: e.ExchangeID != ""},
		CreatedAt:   formatTime(e.CreatedAt),
	})
}

func toExpense(row sqlitedb.Expense) (expense.Expense, error) {
	p, err := period.Parse(row.Period)
	if err != nil {
		return expense.Expense{}, fmt.Errorf("expense row: %w", err)
	}
	currency, err := money.ParseCurrency(row.Currency)
	if err != nil {
		return expense.Expense{}, fmt.Errorf("expense row: %w", err)
	}
	amount, err := money.New(row.AmountMinor, currency)
	if err != nil {
		return expense.Expense{}, fmt.Errorf("expense row: %w", err)
	}
	spentOn, err := time.Parse(time.DateOnly, row.SpentOn)
	if err != nil {
		return expense.Expense{}, fmt.Errorf("expense row: %w", err)
	}
	createdAt, err := parseTime(row.CreatedAt)
	if err != nil {
		return expense.Expense{}, fmt.Errorf("expense row: %w", err)
	}
	return expense.Expense{
		ID:          row.ID,
		Period:      p,
		CategoryID:  row.CategoryID,
		Amount:      amount,
		Description: row.Description,
		SpentOn:     spentOn,
		ExchangeID:  row.ExchangeID.String, // "" when NULL
		CreatedAt:   createdAt,
	}, nil
}
