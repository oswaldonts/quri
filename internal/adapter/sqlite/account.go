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

// SaveTransfer stores a transfer.
func (r *AccountRepository) SaveTransfer(ctx context.Context, t account.Transfer) error {
	return r.q.InsertTransfer(ctx, sqlitedb.InsertTransferParams{
		ID:            t.ID,
		FromAccountID: t.From,
		ToAccountID:   t.To,
		Currency:      string(t.Amount.Currency()),
		AmountMinor:   t.Amount.Amount(),
		Description:   t.Description,
		TransferredOn: t.On.Format(time.DateOnly),
		CreatedAt:     formatTime(t.CreatedAt),
	})
}

// DeleteTransfer returns account.ErrTransferNotFound if no transfer has the ID.
func (r *AccountRepository) DeleteTransfer(ctx context.Context, id string) error {
	n, err := r.q.DeleteTransfer(ctx, id)
	return notFoundIfNone(n, err, account.ErrTransferNotFound, id)
}

// ListTransfers returns every transfer, the latest first.
func (r *AccountRepository) ListTransfers(ctx context.Context) ([]account.Transfer, error) {
	rows, err := r.q.ListTransfers(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]account.Transfer, 0, len(rows))
	for _, row := range rows {
		c, err := money.ParseCurrency(row.Currency)
		if err != nil {
			return nil, fmt.Errorf("transfer row: %w", err)
		}
		amount, err := money.New(row.AmountMinor, c)
		if err != nil {
			return nil, fmt.Errorf("transfer row: %w", err)
		}
		on, err := time.Parse(time.DateOnly, row.TransferredOn)
		if err != nil {
			return nil, fmt.Errorf("transfer row: %w", err)
		}
		createdAt, err := parseTime(row.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("transfer row: %w", err)
		}
		out = append(out, account.Transfer{
			ID: row.ID, From: row.FromAccountID, To: row.ToAccountID,
			Amount: amount, Description: row.Description, On: on, CreatedAt: createdAt,
		})
	}
	return out, nil
}

// movement is an amount with the time it was registered.
type movement struct {
	amountMinor int64
	createdAt   string
}

// MovedSince is what came into the account minus what went out of it since: transfers in and out,
// and the converted amount of the confirmed exchanges that name it as their target.
func (r *AccountRepository) MovedSince(ctx context.Context, accountID string, c money.Currency, since time.Time) (money.Money, error) {
	in, err := r.q.ListTransfersInByAccount(ctx, accountID)
	if err != nil {
		return money.Money{}, err
	}
	out, err := r.q.ListTransfersOutByAccount(ctx, accountID)
	if err != nil {
		return money.Money{}, err
	}
	credits, err := r.q.ListConfirmedExchangeCreditsByAccount(ctx, sql.NullString{String: accountID, Valid: true})
	if err != nil {
		return money.Money{}, err
	}

	total := money.Zero(c)
	sum := func(rows []movement, sign int64) error {
		for _, m := range rows {
			createdAt, err := parseTime(m.createdAt)
			if err != nil {
				return fmt.Errorf("movement row: %w", err)
			}
			if !createdAt.After(since) {
				continue
			}
			amount, err := money.New(sign*m.amountMinor, c)
			if err != nil {
				return fmt.Errorf("movement row: %w", err)
			}
			if total, err = total.Add(amount); err != nil {
				return err
			}
		}
		return nil
	}
	var rows []movement
	for _, row := range in {
		rows = append(rows, movement{row.AmountMinor, row.CreatedAt})
	}
	if err := sum(rows, 1); err != nil {
		return money.Money{}, err
	}
	rows = rows[:0]
	for _, row := range credits {
		rows = append(rows, movement{row.AmountMinor, row.CreatedAt})
	}
	if err := sum(rows, 1); err != nil {
		return money.Money{}, err
	}
	rows = rows[:0]
	for _, row := range out {
		rows = append(rows, movement{row.AmountMinor, row.CreatedAt})
	}
	if err := sum(rows, -1); err != nil {
		return money.Money{}, err
	}
	return total, nil
}

// HasMoves tells whether any transfer or exchange refers to the account.
func (r *AccountRepository) HasMoves(ctx context.Context, accountID string) (bool, error) {
	transfers, err := r.q.AccountHasTransfers(ctx, sqlitedb.AccountHasTransfersParams{FromAccountID: accountID, ToAccountID: accountID})
	if err != nil || transfers {
		return transfers, err
	}
	id := sql.NullString{String: accountID, Valid: true}
	return r.q.AccountHasExchanges(ctx, sqlitedb.AccountHasExchangesParams{FromAccountID: id, ToAccountID: id})
}
