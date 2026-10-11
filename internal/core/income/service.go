package income

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/category"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/currency"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/money"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/period"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/status"
)

const maxDescriptionLength = 200

// Repository is the port the service needs to persist incomes.
// Adapters (in-memory, database...) implement it; the core never imports them.
type Repository interface {
	Save(ctx context.Context, in Income) error
	// Get returns ErrNotFound if no income has the ID.
	Get(ctx context.Context, id string) (Income, error)
	// Update replaces the stored income; it returns ErrNotFound if it doesn't exist.
	Update(ctx context.Context, in Income) error
	// Delete removes the income; it returns ErrNotFound if it doesn't exist.
	Delete(ctx context.Context, id string) error
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
	currencies currency.Checker
	now        func() time.Time
	newID      func() string
}

// NewService returns a Service that stores incomes in repo.
func NewService(repo Repository, categories CategoryResolver, currencies currency.Checker) *Service {
	return &Service{
		repo:       repo,
		categories: categories,
		currencies: currencies,
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
	Status      string // "planned" or "confirmed" (default)
}

// Register validates the input and stores a new income.
func (s *Service) Register(ctx context.Context, in RegisterInput) (Income, error) {
	inc, err := s.build(ctx, in)
	if err != nil {
		return Income{}, err
	}
	if err := s.repo.Save(ctx, inc); err != nil {
		return Income{}, fmt.Errorf("save income: %w", err)
	}
	return inc, nil
}

// UpdateInput is a partial update: nil fields keep their value. An empty
// CategoryID removes the contribution.
type UpdateInput struct {
	Currency    *string
	Amount      *string
	Kind        *string
	Description *string
	ReceivedOn  *string
	CategoryID  *string
	Status      *string
}

// Update changes an income of the period, validating the result like Register.
// Incomes created by exchanges can't be modified: delete the exchange instead.
func (s *Service) Update(ctx context.Context, periodStr, id string, in UpdateInput) (Income, error) {
	current, err := s.editable(ctx, periodStr, id)
	if err != nil {
		return Income{}, err
	}
	merged := RegisterInput{
		Period:      current.Period.String(),
		Currency:    string(current.Amount.Currency()),
		Amount:      current.Amount.String(),
		Kind:        string(current.Kind),
		Description: current.Description,
		ReceivedOn:  current.ReceivedOn.Format(time.DateOnly),
		CategoryID:  current.CategoryID,
		Status:      string(current.Status),
	}
	apply(&merged.Currency, in.Currency)
	apply(&merged.Amount, in.Amount)
	apply(&merged.Kind, in.Kind)
	apply(&merged.Description, in.Description)
	apply(&merged.ReceivedOn, in.ReceivedOn)
	apply(&merged.CategoryID, in.CategoryID)
	apply(&merged.Status, in.Status)

	updated, err := s.build(ctx, merged)
	if err != nil {
		return Income{}, err
	}
	updated.ID, updated.CreatedAt = current.ID, current.CreatedAt
	if err := s.repo.Update(ctx, updated); err != nil {
		return Income{}, fmt.Errorf("update income: %w", err)
	}
	return updated, nil
}

// Delete removes an income of the period. Incomes created by exchanges can't be
// deleted on their own: delete the exchange instead.
func (s *Service) Delete(ctx context.Context, periodStr, id string) error {
	if _, err := s.editable(ctx, periodStr, id); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id)
}

// editable returns the income if it belongs to the period and is not managed by an exchange.
func (s *Service) editable(ctx context.Context, periodStr, id string) (Income, error) {
	p, err := period.Parse(periodStr)
	if err != nil {
		return Income{}, invalid(err)
	}
	inc, err := s.repo.Get(ctx, id)
	if err != nil {
		return Income{}, err
	}
	if inc.Period != p {
		return Income{}, fmt.Errorf("%w: %q in %s", ErrNotFound, id, p)
	}
	if inc.Kind == KindExchange {
		return Income{}, fmt.Errorf("%w: delete the exchange instead", ErrManagedByExchange)
	}
	return inc, nil
}

// apply overwrites dst with *v when v is set.
func apply(dst *string, v *string) {
	if v != nil {
		*dst = *v
	}
}

// build validates the input and returns the income, without saving it.
func (s *Service) build(ctx context.Context, in RegisterInput) (Income, error) {
	p, err := period.Parse(in.Period)
	if err != nil {
		return Income{}, invalid(err)
	}

	currency, err := money.ParseCurrency(in.Currency)
	if err != nil {
		return Income{}, invalid(err)
	}
	if err := s.currencies.Require(ctx, currency); err != nil {
		return Income{}, err
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

	st, err := status.Parse(in.Status)
	if err != nil {
		return Income{}, invalid(err)
	}

	inc := Income{
		ID:          s.newID(),
		Period:      p,
		Amount:      amount,
		Kind:        kind,
		CategoryID:  categoryID,
		Status:      st,
		Description: description,
		ReceivedOn:  receivedOn,
		CreatedAt:   s.now().UTC(),
	}
	return inc, nil
}

// List returns the incomes of a period ordered by date; statusFilter ("planned" or
// "confirmed") keeps only those, empty keeps all.
func (s *Service) List(ctx context.Context, periodStr, statusFilter string) ([]Income, error) {
	p, err := period.Parse(periodStr)
	if err != nil {
		return nil, invalid(err)
	}
	var st status.Status
	if statusFilter != "" {
		if st, err = status.Parse(statusFilter); err != nil {
			return nil, invalid(err)
		}
	}
	all, err := s.repo.ListByPeriod(ctx, p)
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(all, func(in Income) bool { return st != "" && in.Status != st }), nil
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
