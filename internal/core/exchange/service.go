package exchange

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/category"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/expense"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/income"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/money"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/period"
)

const maxDescriptionLength = 200

// RateRepository stores one rate per period and currency pair.
type RateRepository interface {
	// SaveRate creates or replaces the rate for its period and pair.
	SaveRate(ctx context.Context, r PeriodRate) error
	// FindRate returns ErrRateNotFound when no rate was set.
	FindRate(ctx context.Context, p period.Period, from, to money.Currency) (PeriodRate, error)
	// DeleteRate returns ErrRateNotFound when no rate was set.
	DeleteRate(ctx context.Context, p period.Period, from, to money.Currency) error
}

// Repository stores exchanges.
type Repository interface {
	// Save stores the exchange together with the income and the expense it generates,
	// atomically: either all three are saved or none is.
	Save(ctx context.Context, e Exchange, in income.Income, out expense.Expense) error
	// Get returns ErrNotFound if no exchange has the ID.
	Get(ctx context.Context, id string) (Exchange, error)
	// ListByPeriod returns the exchanges of the period ordered by date.
	ListByPeriod(ctx context.Context, p period.Period) ([]Exchange, error)
	// Delete removes the exchange with its income and expense, atomically.
	Delete(ctx context.Context, e Exchange) error
}

// Service holds the rate and exchange use cases.
type Service struct {
	rates      RateRepository
	exchanges  Repository
	categories expense.CategoryResolver
	now        func() time.Time
	newID      func() string
}

// NewService returns a Service. categories provides the system category that
// receives the source side of each exchange.
func NewService(rates RateRepository, exchanges Repository, categories expense.CategoryResolver) *Service {
	return &Service{
		rates:      rates,
		exchanges:  exchanges,
		categories: categories,
		now:        time.Now,
		newID:      rand.Text,
	}
}

// PairInput identifies a currency pair within a period.
type PairInput struct {
	Period string // YYYY-MM
	From   string // ISO 4217 code of the source currency
	To     string // ISO 4217 code of the target currency
}

// SetRateInput carries a new rate for a period and pair.
type SetRateInput struct {
	PairInput
	Rate string // decimal with up to 4 places, e.g. "3.25"
}

// SetRate creates or updates the rate of a period and pair.
// Exchanges already registered keep the rate they were made with.
func (s *Service) SetRate(ctx context.Context, in SetRateInput) (PeriodRate, error) {
	p, from, to, err := parsePair(in.PairInput)
	if err != nil {
		return PeriodRate{}, err
	}
	rate, err := money.ParseRate(in.Rate)
	if err != nil {
		return PeriodRate{}, invalid(err)
	}

	r := PeriodRate{Period: p, From: from, To: to, Rate: rate, UpdatedAt: s.now().UTC()}
	if err := s.rates.SaveRate(ctx, r); err != nil {
		return PeriodRate{}, fmt.Errorf("save rate: %w", err)
	}
	return r, nil
}

// Rate returns the rate of a period and pair, or ErrRateNotFound.
func (s *Service) Rate(ctx context.Context, in PairInput) (PeriodRate, error) {
	p, from, to, err := parsePair(in)
	if err != nil {
		return PeriodRate{}, err
	}
	return s.findRate(ctx, p, from, to)
}

// DeleteRate removes the rate of a period and pair. Exchanges already registered
// keep the rate they were made with.
func (s *Service) DeleteRate(ctx context.Context, in PairInput) error {
	p, from, to, err := parsePair(in)
	if err != nil {
		return err
	}
	if err := s.rates.DeleteRate(ctx, p, from, to); err != nil {
		if errors.Is(err, ErrRateNotFound) {
			return fmt.Errorf("%w: no %s→%s rate set for period %s", ErrRateNotFound, from, to, p)
		}
		return err
	}
	return nil
}

// RegisterInput carries the raw values of a new exchange.
type RegisterInput struct {
	PairInput
	Amount      string // amount spent in the source currency, e.g. "1096.00"
	Description string // optional, up to 200 characters
	ExchangedOn string // YYYY-MM-DD
}

// Register converts Amount with the period rate and stores, in one transaction, the
// exchange, an expense of Amount in the exchange category (source currency) and an
// income of the converted amount (target currency).
func (s *Service) Register(ctx context.Context, in RegisterInput) (Exchange, error) {
	p, from, to, err := parsePair(in.PairInput)
	if err != nil {
		return Exchange{}, err
	}

	amount, err := money.Parse(in.Amount, from)
	if err != nil {
		return Exchange{}, invalid(err)
	}
	if !amount.IsPositive() {
		return Exchange{}, invalid(errors.New("amount must be greater than zero"))
	}

	description := strings.TrimSpace(in.Description)
	if utf8.RuneCountInString(description) > maxDescriptionLength {
		return Exchange{}, invalid(fmt.Errorf("description longer than %d characters", maxDescriptionLength))
	}

	exchangedOn, err := time.Parse(time.DateOnly, in.ExchangedOn)
	if err != nil {
		return Exchange{}, invalid(fmt.Errorf("exchanged_on %q (want YYYY-MM-DD)", in.ExchangedOn))
	}

	pr, err := s.findRate(ctx, p, from, to)
	if err != nil {
		return Exchange{}, err
	}

	converted, err := amount.Convert(pr.Rate, to)
	if err != nil {
		return Exchange{}, invalid(err)
	}
	if !converted.IsPositive() {
		return Exchange{}, invalid(fmt.Errorf("%s %s converts to %s %s", amount, from, converted, to))
	}

	// The source amount is an expense in the system exchange category of the period.
	if _, err := s.categories.EnsureInPeriod(ctx, p, category.ExchangeCategoryID); err != nil {
		return Exchange{}, fmt.Errorf("exchange category: %w", err)
	}

	now := s.now().UTC()
	inc := income.Income{
		ID:          s.newID(),
		Period:      p,
		Amount:      converted,
		Kind:        income.KindExchange,
		Description: fmt.Sprintf("Exchange %s %s at %s", amount, from, pr.Rate),
		ReceivedOn:  exchangedOn,
		CreatedAt:   now,
	}
	ex := Exchange{
		ID:          s.newID(),
		Period:      p,
		From:        amount,
		To:          converted,
		Rate:        pr.Rate,
		Description: description,
		ExchangedOn: exchangedOn,
		IncomeID:    inc.ID,
		CreatedAt:   now,
	}

	out := expense.Expense{
		ID:          s.newID(),
		Period:      p,
		CategoryID:  category.ExchangeCategoryID,
		Amount:      amount,
		Description: fmt.Sprintf("Exchange to %s %s at %s", converted, to, pr.Rate),
		SpentOn:     exchangedOn,
		ExchangeID:  ex.ID,
		CreatedAt:   now,
	}

	if err := s.exchanges.Save(ctx, ex, inc, out); err != nil {
		return Exchange{}, fmt.Errorf("save exchange: %w", err)
	}
	return ex, nil
}

// List returns the exchanges of a period ordered by date.
func (s *Service) List(ctx context.Context, periodStr string) ([]Exchange, error) {
	p, err := period.Parse(periodStr)
	if err != nil {
		return nil, invalid(err)
	}
	return s.exchanges.ListByPeriod(ctx, p)
}

// Delete removes an exchange of the period together with the income and the
// expense it created. To correct an exchange, delete it and register it again.
func (s *Service) Delete(ctx context.Context, periodStr, id string) error {
	p, err := period.Parse(periodStr)
	if err != nil {
		return invalid(err)
	}
	e, err := s.exchanges.Get(ctx, id)
	if err != nil {
		return err
	}
	if e.Period != p {
		return fmt.Errorf("%w: %q in %s", ErrNotFound, id, p)
	}
	return s.exchanges.Delete(ctx, e)
}

func (s *Service) findRate(ctx context.Context, p period.Period, from, to money.Currency) (PeriodRate, error) {
	r, err := s.rates.FindRate(ctx, p, from, to)
	if errors.Is(err, ErrRateNotFound) {
		return PeriodRate{}, fmt.Errorf("%w: no %s→%s rate set for period %s", ErrRateNotFound, from, to, p)
	}
	if err != nil {
		return PeriodRate{}, fmt.Errorf("find rate: %w", err)
	}
	return r, nil
}

// parsePair validates the period and two different supported currencies.
func parsePair(in PairInput) (period.Period, money.Currency, money.Currency, error) {
	p, err := period.Parse(in.Period)
	if err != nil {
		return period.Period{}, "", "", invalid(err)
	}
	from, err := money.ParseCurrency(in.From)
	if err != nil {
		return period.Period{}, "", "", invalid(err)
	}
	to, err := money.ParseCurrency(in.To)
	if err != nil {
		return period.Period{}, "", "", invalid(err)
	}
	if from == to {
		return period.Period{}, "", "", invalid(fmt.Errorf("source and target currency are both %s", from))
	}
	return p, from, to, nil
}

// invalid marks err as a validation error while keeping the original error in the chain.
func invalid(err error) error {
	return fmt.Errorf("%w: %w", ErrInvalidInput, err)
}
