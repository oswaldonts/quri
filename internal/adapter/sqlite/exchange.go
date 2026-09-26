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
	}); err != nil {
		return fmt.Errorf("insert exchange: %w", err)
	}
	if err := insertExpense(ctx, q, out); err != nil {
		return fmt.Errorf("insert expense: %w", err)
	}
	return tx.Commit()
}
