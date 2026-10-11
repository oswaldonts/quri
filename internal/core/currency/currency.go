// Package currency holds the currencies the user works with. The list is managed through the
// API like the accounts: nothing is fixed in code, and every amount must be in a registered one.
package currency

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/money"
)

var (
	// ErrInvalidInput wraps every validation error.
	ErrInvalidInput = errors.New("invalid input")
	// ErrNotFound means the currency is not registered.
	ErrNotFound = errors.New("currency not registered")
	// ErrExists means the currency is already registered.
	ErrExists = errors.New("currency already registered")
	// ErrInUse means amounts exist in the currency, so it can't be removed.
	ErrInUse = errors.New("currency has amounts registered in it")
)

const maxSymbolLength = 4

// Currency is a registered currency.
type Currency struct {
	Code   money.Currency
	Symbol string // shown before amounts, e.g. "S/" or "€"; the code is used when empty
}

// Repository is the port the service needs to persist currencies.
type Repository interface {
	// Save returns ErrExists if the code is already registered.
	Save(ctx context.Context, c Currency) error
	// Get returns ErrNotFound if the code is not registered.
	Get(ctx context.Context, code money.Currency) (Currency, error)
	// Update replaces the symbol; ErrNotFound if the code is not registered.
	Update(ctx context.Context, c Currency) error
	// Delete returns ErrNotFound if the code is not registered.
	Delete(ctx context.Context, code money.Currency) error
	// List returns every currency ordered by code.
	List(ctx context.Context) ([]Currency, error)
	// InUse tells whether any amount (income, expense, account, rate...) is in the currency.
	InUse(ctx context.Context, code money.Currency) (bool, error)
}

// Checker is what the other use cases need: knowing whether a currency can be used.
type Checker interface {
	// Require returns ErrInvalidInput-wrapped ErrNotFound when the code is not registered.
	Require(ctx context.Context, c money.Currency) error
}

// Service holds the currency use cases.
type Service struct {
	repo Repository
}

// NewService returns a Service backed by repo.
func NewService(repo Repository) *Service { return &Service{repo: repo} }

// CreateInput carries the raw values of a new currency.
type CreateInput struct {
	Code   string
	Symbol string
}

// Create registers a currency.
func (s *Service) Create(ctx context.Context, in CreateInput) (Currency, error) {
	code, err := money.ParseCurrency(in.Code)
	if err != nil {
		return Currency{}, invalid(err)
	}
	symbol, err := parseSymbol(in.Symbol)
	if err != nil {
		return Currency{}, err
	}
	c := Currency{Code: code, Symbol: symbol}
	if err := s.repo.Save(ctx, c); err != nil {
		return Currency{}, fmt.Errorf("save currency: %w", err)
	}
	return c, nil
}

// Update changes the symbol of a registered currency.
func (s *Service) Update(ctx context.Context, code string, symbol string) (Currency, error) {
	parsed, err := money.ParseCurrency(code)
	if err != nil {
		return Currency{}, invalid(err)
	}
	sym, err := parseSymbol(symbol)
	if err != nil {
		return Currency{}, err
	}
	c := Currency{Code: parsed, Symbol: sym}
	if err := s.repo.Update(ctx, c); err != nil {
		return Currency{}, fmt.Errorf("update currency: %w", err)
	}
	return c, nil
}

// Delete removes a currency that has no amounts in it.
func (s *Service) Delete(ctx context.Context, code string) error {
	parsed, err := money.ParseCurrency(code)
	if err != nil {
		return invalid(err)
	}
	if _, err := s.repo.Get(ctx, parsed); err != nil {
		return err
	}
	used, err := s.repo.InUse(ctx, parsed)
	if err != nil {
		return fmt.Errorf("check currency use: %w", err)
	}
	if used {
		return fmt.Errorf("%w: %s", ErrInUse, parsed)
	}
	return s.repo.Delete(ctx, parsed)
}

// List returns the registered currencies.
func (s *Service) List(ctx context.Context) ([]Currency, error) {
	out, err := s.repo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list currencies: %w", err)
	}
	return out, nil
}

// Require checks that c is registered.
func (s *Service) Require(ctx context.Context, c money.Currency) error {
	if _, err := s.repo.Get(ctx, c); err != nil {
		if errors.Is(err, ErrNotFound) {
			return invalid(fmt.Errorf("currency %s is not registered: add it in Ajustes first", c))
		}
		return fmt.Errorf("get currency: %w", err)
	}
	return nil
}

func parseSymbol(s string) (string, error) {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) > maxSymbolLength {
		return "", invalid(fmt.Errorf("symbol longer than %d characters", maxSymbolLength))
	}
	return s, nil
}

func invalid(err error) error { return fmt.Errorf("%w: %w", ErrInvalidInput, err) }
