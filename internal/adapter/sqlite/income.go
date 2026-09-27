package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/oswaldoaquinas/go-income-mgr/internal/adapter/sqlite/sqlitedb"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/income"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/money"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/period"
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

// ListByPeriod returns the incomes of the period ordered by date.
func (r *IncomeRepository) ListByPeriod(ctx context.Context, p period.Period) ([]income.Income, error) {
	rows, err := r.q.ListIncomesByPeriod(ctx, p.String())
	if err != nil {
		return nil, err
	}
	out := make([]income.Income, 0, len(rows))
	for _, row := range rows {
		in, err := toIncome(row)
		if err != nil {
			return nil, err
		}
		out = append(out, in)
	}
	return out, nil
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
		CategoryID:  sql.NullString{String: in.CategoryID, Valid: in.CategoryID != ""},
	})
}

func toIncome(row sqlitedb.Income) (income.Income, error) {
	p, err := period.Parse(row.Period)
	if err != nil {
		return income.Income{}, fmt.Errorf("income row: %w", err)
	}
	currency, err := money.ParseCurrency(row.Currency)
	if err != nil {
		return income.Income{}, fmt.Errorf("income row: %w", err)
	}
	amount, err := money.New(row.AmountMinor, currency)
	if err != nil {
		return income.Income{}, fmt.Errorf("income row: %w", err)
	}
	receivedOn, err := time.Parse(time.DateOnly, row.ReceivedOn)
	if err != nil {
		return income.Income{}, fmt.Errorf("income row: %w", err)
	}
	createdAt, err := parseTime(row.CreatedAt)
	if err != nil {
		return income.Income{}, fmt.Errorf("income row: %w", err)
	}
	return income.Income{
		ID:          row.ID,
		Period:      p,
		Amount:      amount,
		Kind:        income.Kind(row.Kind),
		CategoryID:  row.CategoryID.String, // "" when NULL
		Description: row.Description,
		ReceivedOn:  receivedOn,
		CreatedAt:   createdAt,
	}, nil
}

// Get returns income.ErrNotFound if no income has the ID.
func (r *IncomeRepository) Get(ctx context.Context, id string) (income.Income, error) {
	row, err := r.q.GetIncome(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return income.Income{}, fmt.Errorf("%w: %q", income.ErrNotFound, id)
	}
	if err != nil {
		return income.Income{}, err
	}
	return toIncome(row)
}

// Update replaces the editable fields of an income.
func (r *IncomeRepository) Update(ctx context.Context, in income.Income) error {
	n, err := r.q.UpdateIncome(ctx, sqlitedb.UpdateIncomeParams{
		Currency:    string(in.Amount.Currency()),
		AmountMinor: in.Amount.Amount(),
		Kind:        string(in.Kind),
		Description: in.Description,
		ReceivedOn:  in.ReceivedOn.Format(time.DateOnly),
		CategoryID:  sql.NullString{String: in.CategoryID, Valid: in.CategoryID != ""},
		ID:          in.ID,
	})
	return notFoundIfNone(n, err, income.ErrNotFound, in.ID)
}

// Delete removes an income.
func (r *IncomeRepository) Delete(ctx context.Context, id string) error {
	n, err := r.q.DeleteIncome(ctx, id)
	return notFoundIfNone(n, err, income.ErrNotFound, id)
}

// notFoundIfNone turns "no row affected" into the given not-found error.
func notFoundIfNone(rows int64, err error, notFound error, id string) error {
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("%w: %q", notFound, id)
	}
	return nil
}
