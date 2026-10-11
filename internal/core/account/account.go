// Package account tracks where the money is: the cash on hand and the bank accounts.
//
// A balance is typed by hand (to match what the bank shows or what was counted) and goes down
// with the confirmed expenses charged to the account afterwards. Incomes don't credit an account:
// type the new balance when money arrives.
package account

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/money"
)

var (
	// ErrInvalidInput wraps every validation error.
	ErrInvalidInput = errors.New("invalid input")
	// ErrNotFound means no account has the given ID.
	ErrNotFound = errors.New("account not found")
	// ErrNameTaken means the currency already has an account with that name.
	ErrNameTaken = errors.New("account name already used in that currency")
	// ErrInUse means expenses are charged to the account, so it can't be deleted.
	ErrInUse = errors.New("account has expenses charged to it")
	// ErrCashExists means the currency already has its cash account.
	ErrCashExists = errors.New("cash already registered in that currency")
)

const (
	maxNameLength = 40
	cashName      = "Efectivo"
)

// Kind tells how the balance of an account behaves.
type Kind string

const (
	// Bank is an account at a bank.
	Bank Kind = "bank"
	// Cash is the money on hand. Expenses charged to it stay out of the period summary,
	// whose available money is what is in the banks.
	Cash Kind = "cash"
)

func parseKind(s string) (Kind, error) {
	switch Kind(s) {
	case Bank, Cash:
		return Kind(s), nil
	default:
		return "", fmt.Errorf("kind %q (want %q or %q)", s, Bank, Cash)
	}
}

// Account is a place where money is kept, in one currency.
type Account struct {
	ID   string
	Name string
	Kind Kind
	// Amount is what was typed by hand at UpdatedAt.
	Amount money.Money
	// Balance is what is left now: Amount minus the expenses charged to the account since
	// UpdatedAt. It is computed, not stored.
	Balance   money.Money
	UpdatedAt time.Time
}

// Repository is the port the service needs to persist accounts.
type Repository interface {
	Save(ctx context.Context, a Account) error
	// Get returns ErrNotFound if no account has the ID.
	Get(ctx context.Context, id string) (Account, error)
	// Update replaces name, amount and update time; ErrNotFound if it doesn't exist.
	Update(ctx context.Context, a Account) error
	// Delete returns ErrNotFound if no account has the ID.
	Delete(ctx context.Context, id string) error
	// List returns every account ordered by currency, cash first, then by name.
	List(ctx context.Context) ([]Account, error)
}

// Charges reads the expenses charged to the accounts.
type Charges interface {
	// ChargedSince sums the confirmed expenses charged to the account registered after since.
	ChargedSince(ctx context.Context, accountID string, c money.Currency, since time.Time) (money.Money, error)
	// HasExpenses tells whether any expense is charged to the account.
	HasExpenses(ctx context.Context, accountID string) (bool, error)
}

// Service holds the account use cases.
type Service struct {
	repo  Repository
	spent Charges
	now   func() time.Time
	newID func() string
}

// NewService returns a Service backed by repo.
func NewService(repo Repository, spent Charges) *Service {
	return &Service{repo: repo, spent: spent, now: time.Now, newID: rand.Text}
}

// CreateInput carries the raw values of a new account.
type CreateInput struct {
	Name     string // optional for cash: it is "Efectivo"
	Kind     string // "bank" or "cash"
	Currency string // ISO 4217 code
	Balance  string // decimal string, e.g. "200.00"; may be negative (a card debt)
}

// Create validates the input and stores a new account.
func (s *Service) Create(ctx context.Context, in CreateInput) (Account, error) {
	kind, err := parseKind(in.Kind)
	if err != nil {
		return Account{}, invalid(err)
	}
	currency, err := money.ParseCurrency(in.Currency)
	if err != nil {
		return Account{}, invalid(err)
	}
	amount, err := money.Parse(in.Balance, currency)
	if err != nil {
		return Account{}, invalid(err)
	}
	name := strings.TrimSpace(in.Name)
	if name == "" && kind == Cash {
		name = cashName
	}
	if err := checkName(name); err != nil {
		return Account{}, err
	}

	existing, err := s.repo.List(ctx)
	if err != nil {
		return Account{}, fmt.Errorf("list accounts: %w", err)
	}
	for _, other := range existing {
		if other.Amount.Currency() != currency {
			continue
		}
		if kind == Cash && other.Kind == Cash {
			return Account{}, fmt.Errorf("%w: %s", ErrCashExists, currency)
		}
		if strings.EqualFold(other.Name, name) {
			return Account{}, fmt.Errorf("%w: %q in %s", ErrNameTaken, name, currency)
		}
	}

	a := Account{ID: s.newID(), Name: name, Kind: kind, Amount: amount, Balance: amount, UpdatedAt: s.now().UTC()}
	if err := s.repo.Save(ctx, a); err != nil {
		return Account{}, fmt.Errorf("save account: %w", err)
	}
	return a, nil
}

// UpdateInput is a partial update: nil fields keep their value.
type UpdateInput struct {
	Name *string
	// Balance sets what the account holds now. This restarts the count: the expenses
	// registered so far no longer lower it.
	Balance *string
}

// Update renames the account and/or sets its balance.
func (s *Service) Update(ctx context.Context, id string, in UpdateInput) (Account, error) {
	a, err := s.repo.Get(ctx, id)
	if err != nil {
		return Account{}, err
	}
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if err := checkName(name); err != nil {
			return Account{}, err
		}
		if !strings.EqualFold(name, a.Name) {
			existing, err := s.repo.List(ctx)
			if err != nil {
				return Account{}, fmt.Errorf("list accounts: %w", err)
			}
			for _, other := range existing {
				if other.ID != a.ID && other.Amount.Currency() == a.Amount.Currency() && strings.EqualFold(other.Name, name) {
					return Account{}, fmt.Errorf("%w: %q in %s", ErrNameTaken, name, a.Amount.Currency())
				}
			}
		}
		a.Name = name
	}
	if in.Balance != nil {
		amount, err := money.Parse(*in.Balance, a.Amount.Currency())
		if err != nil {
			return Account{}, invalid(err)
		}
		a.Amount, a.UpdatedAt = amount, s.now().UTC()
	}
	if err := s.repo.Update(ctx, a); err != nil {
		return Account{}, fmt.Errorf("update account: %w", err)
	}
	return s.withBalance(ctx, a)
}

// Delete removes an account that has no expenses charged to it.
func (s *Service) Delete(ctx context.Context, id string) error {
	if _, err := s.repo.Get(ctx, id); err != nil {
		return err
	}
	used, err := s.spent.HasExpenses(ctx, id)
	if err != nil {
		return fmt.Errorf("check expenses of account: %w", err)
	}
	if used {
		return ErrInUse
	}
	return s.repo.Delete(ctx, id)
}

// Get returns an account with its current balance.
func (s *Service) Get(ctx context.Context, id string) (Account, error) {
	a, err := s.repo.Get(ctx, id)
	if err != nil {
		return Account{}, err
	}
	return s.withBalance(ctx, a)
}

// List returns every account with its current balance.
func (s *Service) List(ctx context.Context) ([]Account, error) {
	accounts, err := s.repo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	for i := range accounts {
		if accounts[i], err = s.withBalance(ctx, accounts[i]); err != nil {
			return nil, err
		}
	}
	return accounts, nil
}

// withBalance fills Balance: what was typed minus what was charged to the account since.
func (s *Service) withBalance(ctx context.Context, a Account) (Account, error) {
	spent, err := s.spent.ChargedSince(ctx, a.ID, a.Amount.Currency(), a.UpdatedAt)
	if err != nil {
		return Account{}, fmt.Errorf("account charges: %w", err)
	}
	if a.Balance, err = a.Amount.Sub(spent); err != nil {
		return Account{}, fmt.Errorf("account balance: %w", err)
	}
	return a, nil
}

func checkName(name string) error {
	if name == "" {
		return invalid(errors.New("name can't be empty"))
	}
	if utf8.RuneCountInString(name) > maxNameLength {
		return invalid(fmt.Errorf("name longer than %d characters", maxNameLength))
	}
	return nil
}

func invalid(err error) error {
	return fmt.Errorf("%w: %w", ErrInvalidInput, err)
}
