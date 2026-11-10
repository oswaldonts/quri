// Package settings holds the preferences of the installation as a whole.
package settings

import (
	"context"
	"errors"
	"fmt"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/currency"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/money"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/period"
)

// ErrInvalidInput wraps every validation error.
var ErrInvalidInput = errors.New("invalid input")

// Settings are the preferences of the installation.
type Settings struct {
	// SpendOffset is how many months after being earned the money of a period is spent:
	// 0 spends it the same month, 1 the following one (a salary paid at month end).
	SpendOffset int
	// DefaultCurrency is the currency the app opens in; empty means the first one in the list.
	DefaultCurrency string
	// ClosedThrough is the last period the user closed (YYYY-MM, empty: none). The work moves on
	// to the period after it even when the calendar hasn't got there yet.
	ClosedThrough string
}

// Repository is the port the service needs to persist the settings.
type Repository interface {
	Get(ctx context.Context) (Settings, error)
	Save(ctx context.Context, s Settings) error
}

// Service holds the settings use cases.
type Service struct {
	repo       Repository
	currencies currency.Checker
}

// NewService returns a Service backed by repo; currencies validates the default currency.
func NewService(repo Repository, currencies currency.Checker) *Service {
	return &Service{repo: repo, currencies: currencies}
}

// Get returns the current settings.
func (s *Service) Get(ctx context.Context) (Settings, error) {
	cur, err := s.repo.Get(ctx)
	if err != nil {
		return Settings{}, fmt.Errorf("get settings: %w", err)
	}
	return cur, nil
}

// UpdateInput carries the settings to change; nil fields stay as they are.
type UpdateInput struct {
	SpendOffset *int
	// DefaultCurrency "" clears the default.
	DefaultCurrency *string
	// ClosedThrough "" clears it (nothing is closed ahead of the calendar).
	ClosedThrough *string
}

// Update changes the sent settings.
func (s *Service) Update(ctx context.Context, in UpdateInput) (Settings, error) {
	cur, err := s.Get(ctx)
	if err != nil {
		return Settings{}, err
	}
	if in.SpendOffset != nil {
		if *in.SpendOffset < 0 || *in.SpendOffset > 1 {
			return Settings{}, fmt.Errorf("%w: spend_offset %d (want 0 or 1)", ErrInvalidInput, *in.SpendOffset)
		}
		cur.SpendOffset = *in.SpendOffset
	}
	if in.DefaultCurrency != nil {
		cur.DefaultCurrency = ""
		if *in.DefaultCurrency != "" {
			code, err := money.ParseCurrency(*in.DefaultCurrency)
			if err != nil {
				return Settings{}, fmt.Errorf("%w: %w", ErrInvalidInput, err)
			}
			if err := s.currencies.Require(ctx, code); err != nil {
				return Settings{}, err
			}
			cur.DefaultCurrency = string(code)
		}
	}
	if in.ClosedThrough != nil {
		cur.ClosedThrough = ""
		if *in.ClosedThrough != "" {
			p, err := period.Parse(*in.ClosedThrough)
			if err != nil {
				return Settings{}, fmt.Errorf("%w: closed_through: %w", ErrInvalidInput, err)
			}
			cur.ClosedThrough = p.String()
		}
	}
	if err := s.repo.Save(ctx, cur); err != nil {
		return Settings{}, fmt.Errorf("save settings: %w", err)
	}
	return cur, nil
}

// SpendOffset returns just the spend offset.
func (s *Service) SpendOffset(ctx context.Context) (int, error) {
	cur, err := s.Get(ctx)
	return cur.SpendOffset, err
}
