// Package expense registers the money spent in each budget period, by category.
package expense

import (
	"cmp"
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
	// SaveAll stores every expense in one transaction: all or none.
	SaveAll(ctx context.Context, es []Expense) error
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
	e, err := s.parse(in)
	if err != nil {
		return Expense{}, err
	}
	if err := s.checkCategory(ctx, e); err != nil {
		return Expense{}, err
	}
	if err := s.repo.Save(ctx, e); err != nil {
		return Expense{}, fmt.Errorf("save expense: %w", err)
	}
	return e, nil
}

// BatchInput registers several expenses of a period at once, e.g. the items of a
// grocery receipt. Each item inherits the common fields unless it sets its own;
// common tags are added to the item's tags.
type BatchInput struct {
	Period     string
	CategoryID string
	Currency   string
	SpentOn    string
	Tags       []string
	Items      []BatchItem
}

// BatchItem is one line of a batch. Empty fields take the batch's common value.
type BatchItem struct {
	Description string
	Amount      string
	CategoryID  string
	Currency    string
	SpentOn     string
	Tags        []string
}

// maxBatchItems bounds the size of a batch.
const maxBatchItems = 100

// RegisterBatch validates every item first and then stores all of them in one
// transaction: either every item is saved or none is. Errors name the failing item.
func (s *Service) RegisterBatch(ctx context.Context, in BatchInput) ([]Expense, error) {
	if len(in.Items) == 0 {
		return nil, invalid(errors.New("items can't be empty"))
	}
	if len(in.Items) > maxBatchItems {
		return nil, invalid(fmt.Errorf("more than %d items", maxBatchItems))
	}

	expenses := make([]Expense, 0, len(in.Items))
	for i, item := range in.Items {
		e, err := s.parse(RegisterInput{
			Period:      in.Period,
			CategoryID:  cmp.Or(item.CategoryID, in.CategoryID),
			Currency:    cmp.Or(item.Currency, in.Currency),
			Amount:      item.Amount,
			Description: item.Description,
			Tags:        append(slices.Clone(in.Tags), item.Tags...),
			SpentOn:     cmp.Or(item.SpentOn, in.SpentOn),
		})
		if err != nil {
			return nil, fmt.Errorf("item %d: %w", i+1, err)
		}
		expenses = append(expenses, e)
	}
	// Categories are checked once all items are well formed.
	for i, e := range expenses {
		if err := s.checkCategory(ctx, e); err != nil {
			return nil, fmt.Errorf("item %d: %w", i+1, err)
		}
	}

	if err := s.repo.SaveAll(ctx, expenses); err != nil {
		return nil, fmt.Errorf("save expenses: %w", err)
	}
	return expenses, nil
}

// parse validates everything that doesn't need the database and builds the expense.
func (s *Service) parse(in RegisterInput) (Expense, error) {
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
	return Expense{
		ID:          s.newID(),
		Period:      p,
		CategoryID:  strings.TrimSpace(in.CategoryID),
		Amount:      amount,
		Description: description,
		Tags:        tags,
		SpentOn:     spentOn,
		CreatedAt:   s.now().UTC(),
	}, nil
}

// checkCategory verifies that the expense's category is used in its period and is
// not a system category. Using a category fixes the period's category list.
func (s *Service) checkCategory(ctx context.Context, e Expense) error {
	c, err := s.categories.EnsureInPeriod(ctx, e.Period, e.CategoryID)
	if errors.Is(err, category.ErrNotFound) || errors.Is(err, category.ErrNotInPeriod) {
		return invalid(err)
	}
	if err != nil {
		return fmt.Errorf("check category: %w", err)
	}
	if c.IsSystem() {
		return invalid(fmt.Errorf("expenses in %s are created by exchanges", c.Name))
	}
	return nil
}

// Filter narrows a listing; empty fields don't filter.
type Filter struct {
	CategoryID string
	Tag        string // compared after normalization, so "#Dulces" matches "dulces"
}

// List returns the expenses of a period ordered by date, optionally filtered.
func (s *Service) List(ctx context.Context, periodStr string, f Filter) ([]Expense, error) {
	p, err := period.Parse(periodStr)
	if err != nil {
		return nil, invalid(err)
	}
	var tag string
	if strings.TrimSpace(f.Tag) != "" {
		tags, err := NormalizeTags([]string{f.Tag})
		if err != nil {
			return nil, invalid(err)
		}
		tag = tags[0]
	}

	all, err := s.repo.ListByPeriod(ctx, p)
	if err != nil {
		return nil, err
	}
	// A month holds at most a few hundred expenses: filtering in memory is simple and fast enough.
	return slices.DeleteFunc(all, func(e Expense) bool {
		return (f.CategoryID != "" && e.CategoryID != f.CategoryID) ||
			(tag != "" && !slices.Contains(e.Tags, tag))
	}), nil
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
