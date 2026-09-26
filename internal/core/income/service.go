package income

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/category"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/money"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/period"
)

const maxDescriptionLength = 200

// Repository is the port the service needs to persist incomes.
// Adapters (in-memory, database...) implement it; the core never imports them.
type Repository interface {
	Save(ctx context.Context, in Income) error
	// ListByPeriod returns the incomes of the period ordered by date.
	ListByPeriod(ctx context.Context, p period.Period) ([]Income, error)
}

// CategoryResolver checks that a category can be used in a period.
type CategoryResolver interface {
	EnsureInPeriod(ctx context.Context, p period.Period, id string) (category.Category, error)
}

// Service holds the income use cases.
type Service struct {
	repo       Repository
	categories CategoryResolver
	now        func() time.Time
	newID      func() string
}

// NewService returns a Service that stores incomes in repo.
func NewService(repo Repository, categories CategoryResolver) *Service {
	return &Service{
		repo:       repo,
		categories: categories,
		now:        time.Now,
		newID:      rand.Text,
	}
}

// RegisterInput carries the raw values of a new income; Register validates and parses them.
type RegisterInput struct {
	Period      string // YYYY-MM, month the money was earned
	Currency    string // ISO 4217 code
	Amount      string // decimal string, e.g. "1330.00"
	Kind        string // "salary" or "extra"
	Description string // optional, up to 200 characters
	ReceivedOn  string // YYYY-MM-DD
	CategoryID  string // optional, only for "extra": category the money is contributed to
}

// Register validates the input and stores a new income.
func (s *Service) Register(ctx context.Context, in RegisterInput) (Income, error) {
	p, err := period.Parse(in.Period)
	if err != nil {
		return Income{}, invalid(err)
	}

	currency, err := money.ParseCurrency(in.Currency)
	if err != nil {
		return Income{}, invalid(err)
	}

	amount, err := money.Parse(in.Amount, currency)
	if err != nil {
		return Income{}, invalid(err)
	}
	if !amount.IsPositive() {
		return Income{}, invalid(errors.New("amount must be greater than zero"))
	}

	kind, err := parseKind(in.Kind)
	if err != nil {
		return Income{}, invalid(err)
	}

	description := strings.TrimSpace(in.Description)
	if utf8.RuneCountInString(description) > maxDescriptionLength {
		return Income{}, invalid(fmt.Errorf("description longer than %d characters", maxDescriptionLength))
	}

	receivedOn, err := time.Parse(time.DateOnly, in.ReceivedOn)
	if err != nil {
		return Income{}, invalid(fmt.Errorf("received_on %q (want YYYY-MM-DD)", in.ReceivedOn))
	}

	categoryID, err := s.contributionCategory(ctx, p, kind, in.CategoryID)
	if err != nil {
		return Income{}, err
	}

	inc := Income{
		ID:          s.newID(),
		Period:      p,
		Amount:      amount,
		Kind:        kind,
		CategoryID:  categoryID,
		Description: description,
		ReceivedOn:  receivedOn,
		CreatedAt:   s.now().UTC(),
	}
	if err := s.repo.Save(ctx, inc); err != nil {
		return Income{}, fmt.Errorf("save income: %w", err)
	}
	return inc, nil
}

// List returns the incomes of a period ordered by date.
func (s *Service) List(ctx context.Context, periodStr string) ([]Income, error) {
	p, err := period.Parse(periodStr)
	if err != nil {
		return nil, invalid(err)
	}
	return s.repo.ListByPeriod(ctx, p)
}

// contributionCategory validates the optional target category of an extra income.
func (s *Service) contributionCategory(ctx context.Context, p period.Period, kind Kind, id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", nil
	}
	if kind != KindExtra {
		return "", invalid(fmt.Errorf("only %q incomes can be contributed to a category", KindExtra))
	}
	c, err := s.categories.EnsureInPeriod(ctx, p, id)
	if errors.Is(err, category.ErrNotFound) || errors.Is(err, category.ErrNotInPeriod) {
		return "", invalid(err)
	}
	if err != nil {
		return "", fmt.Errorf("check category: %w", err)
	}
	if c.IsSystem() {
		return "", invalid(fmt.Errorf("can't contribute to the system category %s", c.Name))
	}
	return c.ID, nil
}

// invalid marks err as a validation error while keeping the original error in the chain.
func invalid(err error) error {
	return fmt.Errorf("%w: %w", ErrInvalidInput, err)
}
