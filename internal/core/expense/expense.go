// Package expense registers the money spent in each budget period, by category.
package expense

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
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/money"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/period"
)

// ErrInvalidInput wraps every validation error.
var ErrInvalidInput = errors.New("invalid input")

const (
	maxDescriptionLength = 200
	maxTags              = 10
	maxTagLength         = 30
)

// Expense is money spent in a period, in one currency and one category.
type Expense struct {
	ID          string
	Period      period.Period
	CategoryID  string
	Amount      money.Money
	Description string
	Tags        []string // normalized (lowercase, trimmed), unique and sorted
	SpentOn     time.Time
	ExchangeID  string // set when the expense is the source side of a currency exchange
	CreatedAt   time.Time
}

// Repository is the port the service needs to persist expenses.
type Repository interface {
	Save(ctx context.Context, e Expense) error
	// ListByPeriod returns the expenses of the period ordered by date.
	ListByPeriod(ctx context.Context, p period.Period) ([]Expense, error)
}

// CategoryResolver checks that a category can be used in a period.
type CategoryResolver interface {
	EnsureInPeriod(ctx context.Context, p period.Period, id string) (category.Category, error)
}

// Service holds the expense use cases.
type Service struct {
	repo       Repository
	categories CategoryResolver
	now        func() time.Time
	newID      func() string
}

// NewService returns a Service backed by repo.
func NewService(repo Repository, categories CategoryResolver) *Service {
	return &Service{repo: repo, categories: categories, now: time.Now, newID: rand.Text}
}

// RegisterInput carries the raw values of a new expense.
type RegisterInput struct {
	Period      string   // YYYY-MM
	CategoryID  string   // must be used in the period
	Currency    string   // ISO 4217 code
	Amount      string   // decimal string, e.g. "45.50"
	Description string   // optional, up to 200 characters
	Tags        []string // optional kinds of expense, e.g. "dulces"; normalized by NormalizeTags
	SpentOn     string   // YYYY-MM-DD
}

// Register validates the input and stores a new expense.
func (s *Service) Register(ctx context.Context, in RegisterInput) (Expense, error) {
	p, err := period.Parse(in.Period)
	if err != nil {
		return Expense{}, invalid(err)
	}
	currency, err := money.ParseCurrency(in.Currency)
	if err != nil {
		return Expense{}, invalid(err)
	}
	amount, err := money.Parse(in.Amount, currency)
	if err != nil {
		return Expense{}, invalid(err)
	}
	if !amount.IsPositive() {
		return Expense{}, invalid(errors.New("amount must be greater than zero"))
	}
	description := strings.TrimSpace(in.Description)
	if utf8.RuneCountInString(description) > maxDescriptionLength {
		return Expense{}, invalid(fmt.Errorf("description longer than %d characters", maxDescriptionLength))
	}
	spentOn, err := time.Parse(time.DateOnly, in.SpentOn)
	if err != nil {
		return Expense{}, invalid(fmt.Errorf("spent_on %q (want YYYY-MM-DD)", in.SpentOn))
	}
	tags, err := NormalizeTags(in.Tags)
	if err != nil {
		return Expense{}, invalid(err)
	}

	c, err := s.categories.EnsureInPeriod(ctx, p, in.CategoryID)
	if errors.Is(err, category.ErrNotFound) || errors.Is(err, category.ErrNotInPeriod) {
		return Expense{}, invalid(err)
	}
	if err != nil {
		return Expense{}, fmt.Errorf("check category: %w", err)
	}
	if c.IsSystem() {
		return Expense{}, invalid(fmt.Errorf("expenses in %s are created by exchanges", c.Name))
	}

	e := Expense{
		ID:          s.newID(),
		Period:      p,
		CategoryID:  c.ID,
		Amount:      amount,
		Description: description,
		Tags:        tags,
		SpentOn:     spentOn,
		CreatedAt:   s.now().UTC(),
	}
	if err := s.repo.Save(ctx, e); err != nil {
		return Expense{}, fmt.Errorf("save expense: %w", err)
	}
	return e, nil
}

// List returns the expenses of a period ordered by date.
func (s *Service) List(ctx context.Context, periodStr string) ([]Expense, error) {
	p, err := period.Parse(periodStr)
	if err != nil {
		return nil, invalid(err)
	}
	return s.repo.ListByPeriod(ctx, p)
}

// NormalizeTags lowercases and trims each tag, drops a leading "#", collapses inner
// spaces, removes duplicates and sorts: ["#Dulces", "dulces ", "Snacks"] → ["dulces", "snacks"].
func NormalizeTags(raw []string) ([]string, error) {
	tags := make([]string, 0, len(raw))
	for _, r := range raw {
		tag := strings.ToLower(strings.Join(strings.Fields(strings.TrimPrefix(strings.TrimSpace(r), "#")), " "))
		if tag == "" {
			return nil, errors.New("tags can't be empty")
		}
		if utf8.RuneCountInString(tag) > maxTagLength {
			return nil, fmt.Errorf("tag %q longer than %d characters", tag, maxTagLength)
		}
		tags = append(tags, tag)
	}
	slices.Sort(tags)
	tags = slices.Compact(tags)
	if len(tags) > maxTags {
		return nil, fmt.Errorf("more than %d tags", maxTags)
	}
	return tags, nil
}

func invalid(err error) error {
	return fmt.Errorf("%w: %w", ErrInvalidInput, err)
}
