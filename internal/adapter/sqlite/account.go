package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"github.com/oswaldoaquinas/go-income-mgr/internal/adapter/sqlite/sqlitedb"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/account"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/money"
)

// Compile-time check that AccountRepository implements the core port.
var _ account.Repository = (*AccountRepository)(nil)

// AccountRepository stores the cash and bank accounts in SQLite.
type AccountRepository struct {
	q *sqlitedb.Queries
}

// NewAccountRepository returns a repository backed by db.
func NewAccountRepository(db *sql.DB) *AccountRepository {
	return &AccountRepository{q: sqlitedb.New(db)}
}

// Save inserts the account; a repeated name or a second cash in a currency is a conflict.
func (r *AccountRepository) Save(ctx context.Context, a account.Account) error {
	err := r.q.InsertAccount(ctx, sqlitedb.InsertAccountParams{
		ID:          a.ID,
		Name:        a.Name,
		Kind:        string(a.Kind),
		Currency:    string(a.Amount.Currency()),
		AmountMinor: a.Amount.Amount(),
		UpdatedAt:   formatTime(a.UpdatedAt),
	})
	var sqlErr *sqlite.Error
	if errors.As(err, &sqlErr) && sqlErr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE {
		return account.ErrNameTaken
	}
	return err
}

// Get returns account.ErrNotFound if no account has the ID.
func (r *AccountRepository) Get(ctx context.Context, id string) (account.Account, error) {
	row, err := r.q.GetAccount(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return account.Account{}, fmt.Errorf("%w: %q", account.ErrNotFound, id)
	}
	if err != nil {
		return account.Account{}, err
	}
	return toAccount(row)
}

// Update replaces the name, amount and update time of an account.
func (r *AccountRepository) Update(ctx context.Context, a account.Account) error {
	n, err := r.q.UpdateAccount(ctx, sqlitedb.UpdateAccountParams{
		Name:        a.Name,
		AmountMinor: a.Amount.Amount(),
		UpdatedAt:   formatTime(a.UpdatedAt),
		ID:          a.ID,
	})
	return notFoundIfNone(n, err, account.ErrNotFound, a.ID)
}

// Delete removes an account.
func (r *AccountRepository) Delete(ctx context.Context, id string) error {
	n, err := r.q.DeleteAccount(ctx, id)
	return notFoundIfNone(n, err, account.ErrNotFound, id)
}

// List returns every account ordered by currency, cash first, then by name.
func (r *AccountRepository) List(ctx context.Context) ([]account.Account, error) {
	rows, err := r.q.ListAccounts(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]account.Account, 0, len(rows))
	for _, row := range rows {
		a, err := toAccount(row)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}

func toAccount(row sqlitedb.Account) (account.Account, error) {
	currency, err := money.ParseCurrency(row.Currency)
	if err != nil {
		return account.Account{}, fmt.Errorf("account row: %w", err)
	}
	amount, err := money.New(row.AmountMinor, currency)
	if err != nil {
		return account.Account{}, fmt.Errorf("account row: %w", err)
	}
	updatedAt, err := parseTime(row.UpdatedAt)
	if err != nil {
		return account.Account{}, fmt.Errorf("account row: %w", err)
	}
	return account.Account{
		ID:        row.ID,
		Name:      row.Name,
		Kind:      account.Kind(row.Kind),
		Amount:    amount,
		Balance:   amount,
		UpdatedAt: updatedAt,
	}, nil
}

// ChargedSince sums the confirmed expenses charged to the account that were registered after since.
func (r *ExpenseRepository) ChargedSince(ctx context.Context, accountID string, c money.Currency, since time.Time) (money.Money, error) {
	rows, err := r.q.ListConfirmedExpensesByAccount(ctx, sql.NullString{String: accountID, Valid: true})
	if err != nil {
		return money.Money{}, err
	}
	total := money.Zero(c)
	for _, row := range rows {
		createdAt, err := parseTime(row.CreatedAt)
		if err != nil {
			return money.Money{}, fmt.Errorf("expense row: %w", err)
		}
		if !createdAt.After(since) {
			continue
		}
		amount, err := money.New(row.AmountMinor, c)
		if err != nil {
			return money.Money{}, fmt.Errorf("expense row: %w", err)
		}
		if total, err = total.Add(amount); err != nil {
			return money.Money{}, err
		}
	}
	return total, nil
}

// HasExpenses tells whether any expense is charged to the account.
func (r *ExpenseRepository) HasExpenses(ctx context.Context, accountID string) (bool, error) {
	return r.q.AccountHasExpenses(ctx, sql.NullString{String: accountID, Valid: true})
}
