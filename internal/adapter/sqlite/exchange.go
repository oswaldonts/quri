package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/oswaldoaquinas/go-income-mgr/internal/adapter/sqlite/sqlitedb"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/exchange"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/expense"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/income"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/money"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/period"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/status"
)

// Compile-time checks that the repositories implement the core ports.
var (
	_ exchange.RateRepository = (*RateRepository)(nil)
	_ exchange.Repository     = (*ExchangeRepository)(nil)
)

// RateRepository stores period rates in SQLite.
type RateRepository struct {
	q *sqlitedb.Queries
}

// NewRateRepository returns a repository backed by db.
func NewRateRepository(db *sql.DB) *RateRepository {
	return &RateRepository{q: sqlitedb.New(db)}
}

// SaveRate inserts the rate or replaces the existing one for its period and pair.
func (r *RateRepository) SaveRate(ctx context.Context, pr exchange.PeriodRate) error {
	return r.q.UpsertRate(ctx, sqlitedb.UpsertRateParams{
		Period:       pr.Period.String(),
		FromCurrency: string(pr.From),
		ToCurrency:   string(pr.To),
		RateScaled:   pr.Rate.Scaled(),
		UpdatedAt:    formatTime(pr.UpdatedAt),
	})
}

// FindRate returns exchange.ErrRateNotFound when the pair has no rate in the period.
func (r *RateRepository) FindRate(ctx context.Context, p period.Period, from, to money.Currency) (exchange.PeriodRate, error) {
	row, err := r.q.GetRate(ctx, sqlitedb.GetRateParams{
		Period:       p.String(),
		FromCurrency: string(from),
		ToCurrency:   string(to),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return exchange.PeriodRate{}, exchange.ErrRateNotFound
	}
	if err != nil {
		return exchange.PeriodRate{}, err
	}
	return toPeriodRate(row)
}

// DeleteRate removes the rate of a period and pair.
func (r *RateRepository) DeleteRate(ctx context.Context, p period.Period, from, to money.Currency) error {
	n, err := r.q.DeleteRate(ctx, sqlitedb.DeleteRateParams{Period: p.String(), FromCurrency: string(from), ToCurrency: string(to)})
	if err != nil {
		return err
	}
	if n == 0 {
		return exchange.ErrRateNotFound
	}
	return nil
}

// toPeriodRate rebuilds the domain type from a row, validating every column.
func toPeriodRate(row sqlitedb.Rate) (exchange.PeriodRate, error) {
	p, err := period.Parse(row.Period)
	if err != nil {
		return exchange.PeriodRate{}, fmt.Errorf("rate row: %w", err)
	}
	from, err := money.ParseCurrency(row.FromCurrency)
	if err != nil {
		return exchange.PeriodRate{}, fmt.Errorf("rate row: %w", err)
	}
	to, err := money.ParseCurrency(row.ToCurrency)
	if err != nil {
		return exchange.PeriodRate{}, fmt.Errorf("rate row: %w", err)
	}
	rate, err := money.NewRate(row.RateScaled)
	if err != nil {
		return exchange.PeriodRate{}, fmt.Errorf("rate row: %w", err)
	}
	updatedAt, err := parseTime(row.UpdatedAt)
	if err != nil {
		return exchange.PeriodRate{}, fmt.Errorf("rate row: %w", err)
	}
	return exchange.PeriodRate{Period: p, From: from, To: to, Rate: rate, UpdatedAt: updatedAt}, nil
}

// ExchangeRepository stores exchanges in SQLite.
type ExchangeRepository struct {
	db *sql.DB
}

// NewExchangeRepository returns a repository backed by db.
func NewExchangeRepository(db *sql.DB) *ExchangeRepository {
	return &ExchangeRepository{db: db}
}

// Save inserts the exchange, its income and its expense in a single transaction.
func (r *ExchangeRepository) Save(ctx context.Context, e exchange.Exchange, in income.Income, out expense.Expense) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	// Rollback is a no-op once Commit succeeds; on any early return it undoes everything.
	defer func() { _ = tx.Rollback() }()

	q := sqlitedb.New(tx)
	// Order follows the foreign keys: exchanges.income_id → incomes, expenses.exchange_id → exchanges.
	if err := insertIncome(ctx, q, in); err != nil {
		return fmt.Errorf("insert income: %w", err)
	}
	if err := q.InsertExchange(ctx, sqlitedb.InsertExchangeParams{
		ID:              e.ID,
		Period:          e.Period.String(),
		FromCurrency:    string(e.From.Currency()),
		FromAmountMinor: e.From.Amount(),
		ToCurrency:      string(e.To.Currency()),
		ToAmountMinor:   e.To.Amount(),
		RateScaled:      e.Rate.Scaled(),
		Description:     e.Description,
		ExchangedOn:     e.ExchangedOn.Format(time.DateOnly),
		IncomeID:        e.IncomeID,
		CreatedAt:       formatTime(e.CreatedAt),
		Status:          storedStatus(e.Status),
		FromAccountID:   sql.NullString{String: e.FromAccountID, Valid: e.FromAccountID != ""},
		ToAccountID:     sql.NullString{String: e.ToAccountID, Valid: e.ToAccountID != ""},
	}); err != nil {
		return fmt.Errorf("insert exchange: %w", err)
	}
	if err := insertExpense(ctx, q, out); err != nil {
		return fmt.Errorf("insert expense: %w", err)
	}
	return tx.Commit()
}

// Get returns exchange.ErrNotFound if no exchange has the ID.
func (r *ExchangeRepository) Get(ctx context.Context, id string) (exchange.Exchange, error) {
	row, err := sqlitedb.New(r.db).GetExchange(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return exchange.Exchange{}, fmt.Errorf("%w: %q", exchange.ErrNotFound, id)
	}
	if err != nil {
		return exchange.Exchange{}, err
	}
	return toExchange(row)
}

// ListByPeriod returns the exchanges of the period ordered by date.
func (r *ExchangeRepository) ListByPeriod(ctx context.Context, p period.Period) ([]exchange.Exchange, error) {
	rows, err := sqlitedb.New(r.db).ListExchangesByPeriod(ctx, p.String())
	if err != nil {
		return nil, err
	}
	out := make([]exchange.Exchange, 0, len(rows))
	for _, row := range rows {
		e, err := toExchange(row)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

// Update replaces the exchange, its income and its expense in one transaction.
func (r *ExchangeRepository) Update(ctx context.Context, e exchange.Exchange, in income.Income, out expense.Expense) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	q := sqlitedb.New(tx)
	n, err := q.UpdateExchange(ctx, sqlitedb.UpdateExchangeParams{
		FromAmountMinor: e.From.Amount(),
		ToAmountMinor:   e.To.Amount(),
		Description:     e.Description,
		ExchangedOn:     e.ExchangedOn.Format(time.DateOnly),
		Status:          storedStatus(e.Status),
		FromAccountID:   sql.NullString{String: e.FromAccountID, Valid: e.FromAccountID != ""},
		ToAccountID:     sql.NullString{String: e.ToAccountID, Valid: e.ToAccountID != ""},
		ID:              e.ID,
	})
	if err := notFoundIfNone(n, err, exchange.ErrNotFound, e.ID); err != nil {
		return err
	}
	if _, err := q.UpdateIncome(ctx, sqlitedb.UpdateIncomeParams{
		Currency:    string(in.Amount.Currency()),
		AmountMinor: in.Amount.Amount(),
		Kind:        string(in.Kind),
		Description: in.Description,
		ReceivedOn:  in.ReceivedOn.Format(time.DateOnly),
		Status:      storedStatus(in.Status),
		ID:          in.ID,
	}); err != nil {
		return fmt.Errorf("update income: %w", err)
	}
	if err := q.UpdateExchangeExpense(ctx, sqlitedb.UpdateExchangeExpenseParams{
		AmountMinor: out.Amount.Amount(),
		Description: out.Description,
		SpentOn:     out.SpentOn.Format(time.DateOnly),
		Status:      storedStatus(out.Status),
		AccountID:   sql.NullString{String: out.AccountID, Valid: out.AccountID != ""},
		ExchangeID:  sql.NullString{String: e.ID, Valid: true},
	}); err != nil {
		return fmt.Errorf("update expense: %w", err)
	}
	return tx.Commit()
}

// Delete removes the exchange with its expense and income in one transaction,
// in the order the foreign keys require.
func (r *ExchangeRepository) Delete(ctx context.Context, e exchange.Exchange) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	q := sqlitedb.New(tx)
	if err := q.DeleteExchangeExpense(ctx, sql.NullString{String: e.ID, Valid: true}); err != nil {
		return fmt.Errorf("delete expense: %w", err)
	}
	if err := q.DeleteExchange(ctx, e.ID); err != nil {
		return fmt.Errorf("delete exchange: %w", err)
	}
	if _, err := q.DeleteIncome(ctx, e.IncomeID); err != nil {
		return fmt.Errorf("delete income: %w", err)
	}
	return tx.Commit()
}

func toExchange(row sqlitedb.Exchange) (exchange.Exchange, error) {
	p, err := period.Parse(row.Period)
	if err != nil {
		return exchange.Exchange{}, fmt.Errorf("exchange row: %w", err)
	}
	fromCurrency, err := money.ParseCurrency(row.FromCurrency)
	if err != nil {
		return exchange.Exchange{}, fmt.Errorf("exchange row: %w", err)
	}
	from, err := money.New(row.FromAmountMinor, fromCurrency)
	if err != nil {
		return exchange.Exchange{}, fmt.Errorf("exchange row: %w", err)
	}
	toCurrency, err := money.ParseCurrency(row.ToCurrency)
	if err != nil {
		return exchange.Exchange{}, fmt.Errorf("exchange row: %w", err)
	}
	to, err := money.New(row.ToAmountMinor, toCurrency)
	if err != nil {
		return exchange.Exchange{}, fmt.Errorf("exchange row: %w", err)
	}
	rate, err := money.NewRate(row.RateScaled)
	if err != nil {
		return exchange.Exchange{}, fmt.Errorf("exchange row: %w", err)
	}
	exchangedOn, err := time.Parse(time.DateOnly, row.ExchangedOn)
	if err != nil {
		return exchange.Exchange{}, fmt.Errorf("exchange row: %w", err)
	}
	createdAt, err := parseTime(row.CreatedAt)
	if err != nil {
		return exchange.Exchange{}, fmt.Errorf("exchange row: %w", err)
	}
	return exchange.Exchange{
		ID:          row.ID,
		Period:      p,
		From:        from,
		To:          to,
		Rate:        rate,
		Description: row.Description,
		ExchangedOn: exchangedOn,
		IncomeID:    row.IncomeID,
		Status:      status.Status(row.Status),
		CreatedAt:   createdAt,

		FromAccountID: row.FromAccountID.String,
		ToAccountID:   row.ToAccountID.String,
	}, nil
}
