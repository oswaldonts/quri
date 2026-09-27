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

// ExpenseRepository stores expenses and their tags in SQLite.
type ExpenseRepository struct {
	db *sql.DB
	q  *sqlitedb.Queries
}

// NewExpenseRepository returns a repository backed by db.
func NewExpenseRepository(db *sql.DB) *ExpenseRepository {
	return &ExpenseRepository{db: db, q: sqlitedb.New(db)}
}

// Save inserts the expense and its tags in one transaction.
func (r *ExpenseRepository) Save(ctx context.Context, e expense.Expense) error {
	return r.SaveAll(ctx, []expense.Expense{e})
}

// SaveAll inserts every expense with its tags in one transaction: all or none.
func (r *ExpenseRepository) SaveAll(ctx context.Context, es []expense.Expense) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	q := r.q.WithTx(tx)
	for _, e := range es {
		if err := insertExpense(ctx, q, e); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ListByPeriod returns the expenses of the period ordered by date, with their tags.
func (r *ExpenseRepository) ListByPeriod(ctx context.Context, p period.Period) ([]expense.Expense, error) {
	rows, err := r.q.ListExpensesByPeriod(ctx, p.String())
	if err != nil {
		return nil, err
	}
	tagRows, err := r.q.ListExpenseTagsByPeriod(ctx, p.String())
	if err != nil {
		return nil, err
	}
	// Two queries instead of one per expense: all tags of the period, grouped in memory.
	tags := make(map[string][]string)
	for _, t := range tagRows {
		tags[t.ExpenseID] = append(tags[t.ExpenseID], t.Tag)
	}

	out := make([]expense.Expense, 0, len(rows))
	for _, row := range rows {
		e, err := toExpense(row)
		if err != nil {
			return nil, err
		}
		e.Tags = tags[e.ID]
		out = append(out, e)
	}
	return out, nil
}

// insertExpense inserts the expense and its tags. It is shared with ExchangeRepository;
// callers run it inside a transaction.
func insertExpense(ctx context.Context, q *sqlitedb.Queries, e expense.Expense) error {
	if err := q.InsertExpense(ctx, sqlitedb.InsertExpenseParams{
		ID:          e.ID,
		Period:      e.Period.String(),
		CategoryID:  e.CategoryID,
		Currency:    string(e.Amount.Currency()),
		AmountMinor: e.Amount.Amount(),
		Description: e.Description,
		SpentOn:     e.SpentOn.Format(time.DateOnly),
		ExchangeID:  sql.NullString{String: e.ExchangeID, Valid: e.ExchangeID != ""},
		CreatedAt:   formatTime(e.CreatedAt),
	}); err != nil {
		return fmt.Errorf("insert expense: %w", err)
	}
	for _, tag := range e.Tags {
		if err := q.InsertExpenseTag(ctx, sqlitedb.InsertExpenseTagParams{ExpenseID: e.ID, Tag: tag}); err != nil {
			return fmt.Errorf("insert tag %q: %w", tag, err)
		}
	}
	return nil
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
