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

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/account"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/category"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/currency"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/money"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/period"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/status"
)

var (
	// ErrInvalidInput wraps every validation error.
	ErrInvalidInput = errors.New("invalid input")
	// ErrNotFound means no expense of the period has the given ID.
	ErrNotFound = errors.New("expense not found")
	// ErrManagedByExchange means the expense was created by an exchange and can only
	// change through it.
	ErrManagedByExchange = errors.New("expense is managed by its exchange")
)

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
	ExchangeID  string        // set when the expense is the source side of a currency exchange
	Status      status.Status // planned or confirmed
	// AccountID is the account the money came out of ("" when not tracked).
	AccountID string
	// Disabled keeps the expense registered but out of every sum: summary and account balance.
	Disabled bool
	// SeriesID links the monthly occurrences of a recurring expense ("" when it happens once).
	// SeriesNo is this occurrence's position (1-based) out of SeriesTotal.
	SeriesID              string
	SeriesNo, SeriesTotal int
	CreatedAt             time.Time
}

// Repository is the port the service needs to persist expenses.
type Repository interface {
	Save(ctx context.Context, e Expense) error
	// SaveAll stores every expense in one transaction: all or none.
	SaveAll(ctx context.Context, es []Expense) error
	// Get returns ErrNotFound if no expense has the ID.
	Get(ctx context.Context, id string) (Expense, error)
	// Update replaces the stored expense and its tags; ErrNotFound if it doesn't exist.
	Update(ctx context.Context, e Expense) error
	// Delete removes the expense and its tags; ErrNotFound if it doesn't exist.
	Delete(ctx context.Context, id string) error
	// ListByPeriod returns the expenses of the period ordered by date.
	ListByPeriod(ctx context.Context, p period.Period) ([]Expense, error)
	// StartSeries stores first's series fields (none clears them) and inserts the
	// following occurrences, in one transaction; ErrNotFound if first doesn't exist.
	StartSeries(ctx context.Context, first Expense, following []Expense) error
	// DeleteSeriesFrom removes the occurrences of the series from the period on.
	DeleteSeriesFrom(ctx context.Context, seriesID string, from period.Period) error
}

// CategoryResolver checks that a category can be used in a period.
type CategoryResolver interface {
	EnsureInPeriod(ctx context.Context, p period.Period, id string) (category.Category, error)
}

// AccountResolver finds the account an expense is charged to.
type AccountResolver interface {
	// Get returns account.ErrNotFound if no account has the ID.
	Get(ctx context.Context, id string) (account.Account, error)
}

// Service holds the expense use cases.
type Service struct {
	repo       Repository
	categories CategoryResolver
	accounts   AccountResolver
	currencies currency.Checker
	now        func() time.Time
	newID      func() string
}

// NewService returns a Service backed by repo.
func NewService(repo Repository, categories CategoryResolver, accounts AccountResolver, currencies currency.Checker) *Service {
	return &Service{repo: repo, categories: categories, accounts: accounts, currencies: currencies, now: time.Now, newID: rand.Text}
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
	Status      string   // "planned" or "confirmed" (default)
	AccountID   string   // optional account the money comes out of; same currency
	// Occurrences is how many consecutive months the expense appears (0 or 1: only this one).
	// The following ones are planned expenses in the next periods, same amount and category.
	Occurrences int
	// FirstNo is the installment this expense is within the Occurrences (0 or 1: the first). An
	// expense that is already installment 9 of 12 only creates the 3 remaining ones.
	FirstNo int
}

// MaxOccurrences bounds how many months a recurring expense can span.
const MaxOccurrences = 60

// Register validates the input and stores a new expense, plus its following occurrences
// when it recurs.
func (s *Service) Register(ctx context.Context, in RegisterInput) (Expense, error) {
	e, err := s.parse(in)
	if err != nil {
		return Expense{}, err
	}
	if err := s.checkCategory(ctx, e); err != nil {
		return Expense{}, err
	}
	if err := s.currencies.Require(ctx, e.Amount.Currency()); err != nil {
		return Expense{}, err
	}
	if err := s.chargeAccount(ctx, &e); err != nil {
		return Expense{}, err
	}
	if in.Occurrences < 0 || in.FirstNo < 0 {
		return Expense{}, invalid(errors.New("occurrences and first_no can't be negative"))
	}
	total, firstNo := max(in.Occurrences, 1), max(in.FirstNo, 1)
	if err := checkInstallments(firstNo, total); err != nil {
		return Expense{}, err
	}
	if total == 1 {
		if err := s.repo.Save(ctx, e); err != nil {
			return Expense{}, fmt.Errorf("save expense: %w", err)
		}
		return e, nil
	}
	following, err := s.startSeries(ctx, &e, firstNo, total)
	if err != nil {
		return Expense{}, err
	}
	if err := s.repo.SaveAll(ctx, append([]Expense{e}, following...)); err != nil {
		return Expense{}, fmt.Errorf("save expenses: %w", err)
	}
	return e, nil
}

func checkInstallments(no, total int) error {
	if total < 1 || total > MaxOccurrences || no < 1 || no > total {
		return invalid(fmt.Errorf("installment %d of %d: want 1 <= installment <= total <= %d", no, total, MaxOccurrences))
	}
	return nil
}

// startSeries makes first installment no of total and returns the ones after it, planned in the
// following periods on the same day of the month (the last day when the month is shorter).
func (s *Service) startSeries(ctx context.Context, first *Expense, no, total int) ([]Expense, error) {
	first.SeriesID, first.SeriesNo, first.SeriesTotal = s.newID(), no, total
	following := make([]Expense, 0, total-no)
	for i := 1; i <= total-no; i++ {
		next := *first
		next.ID = s.newID()
		next.Period = first.Period.Shift(i)
		next.SpentOn = addMonths(first.SpentOn, i)
		next.Status = status.Planned
		next.Disabled = false
		next.SeriesNo = no + i
		next.Tags = slices.Clone(first.Tags)
		if err := s.checkCategory(ctx, next); err != nil {
			return nil, fmt.Errorf("installment %d: %w", no+i, err)
		}
		following = append(following, next)
	}
	return following, nil
}

// addMonths moves a date n months ahead, keeping the day or the last day of a shorter month.
func addMonths(d time.Time, n int) time.Time {
	first := time.Date(d.Year(), d.Month()+time.Month(n), 1, 0, 0, 0, 0, time.UTC)
	lastDay := first.AddDate(0, 1, -1).Day()
	return time.Date(first.Year(), first.Month(), min(d.Day(), lastDay), 0, 0, 0, 0, time.UTC)
}

// Repeat sets an expense as installment firstNo of total, creating the ones after it. An expense
// that already repeats is reset: its later installments are replaced by the new ones, and a
// total of 1 makes it a plain expense again.
func (s *Service) Repeat(ctx context.Context, periodStr, id string, firstNo, total int) (Expense, error) {
	first, err := s.editable(ctx, periodStr, id)
	if err != nil {
		return Expense{}, err
	}
	if err := checkInstallments(firstNo, total); err != nil {
		return Expense{}, err
	}
	oldSeries := first.SeriesID
	var following []Expense
	switch {
	case total > 1:
		if following, err = s.startSeries(ctx, &first, firstNo, total); err != nil {
			return Expense{}, err
		}
	case oldSeries != "":
		first.SeriesID, first.SeriesNo, first.SeriesTotal = "", 0, 0
	default:
		return Expense{}, invalid(errors.New("a recurring expense has at least 2 installments"))
	}
	if oldSeries != "" {
		if err := s.repo.DeleteSeriesFrom(ctx, oldSeries, first.Period.Next()); err != nil {
			return Expense{}, fmt.Errorf("replace series: %w", err)
		}
	}
	if err := s.repo.StartSeries(ctx, first, following); err != nil {
		return Expense{}, fmt.Errorf("start series: %w", err)
	}
	return first, nil
}

// DeleteFollowing removes the expense and the occurrences of its series in later periods.
// An expense that doesn't repeat is deleted alone.
func (s *Service) DeleteFollowing(ctx context.Context, periodStr, id string) error {
	e, err := s.editable(ctx, periodStr, id)
	if err != nil {
		return err
	}
	if e.SeriesID == "" {
		return s.repo.Delete(ctx, id)
	}
	return s.repo.DeleteSeriesFrom(ctx, e.SeriesID, e.Period)
}

// UpdateInput is a partial update: nil fields keep their value. Tags, when set,
// replace all the current tags.
type UpdateInput struct {
	CategoryID  *string
	Currency    *string
	Amount      *string
	Description *string
	Tags        *[]string
	SpentOn     *string
	Status      *string
	AccountID   *string // "" removes the account
	Disabled    *bool
}

// Update changes an expense of the period, validating the result like Register.
// Expenses created by exchanges can't be modified: delete the exchange instead.
func (s *Service) Update(ctx context.Context, periodStr, id string, in UpdateInput) (Expense, error) {
	current, err := s.editable(ctx, periodStr, id)
	if err != nil {
		return Expense{}, err
	}
	merged := RegisterInput{
		Period:      current.Period.String(),
		CategoryID:  current.CategoryID,
		Currency:    string(current.Amount.Currency()),
		Amount:      current.Amount.String(),
		Description: current.Description,
		Tags:        current.Tags,
		SpentOn:     current.SpentOn.Format(time.DateOnly),
		Status:      string(current.Status),
		AccountID:   current.AccountID,
	}
	apply(&merged.CategoryID, in.CategoryID)
	apply(&merged.Currency, in.Currency)
	apply(&merged.Amount, in.Amount)
	apply(&merged.Description, in.Description)
	apply(&merged.SpentOn, in.SpentOn)
	apply(&merged.Tags, in.Tags)
	apply(&merged.Status, in.Status)
	apply(&merged.AccountID, in.AccountID)

	updated, err := s.parse(merged)
	if err != nil {
		return Expense{}, err
	}
	if err := s.checkCategory(ctx, updated); err != nil {
		return Expense{}, err
	}
	if err := s.chargeAccount(ctx, &updated); err != nil {
		return Expense{}, err
	}
	updated.ID, updated.CreatedAt = current.ID, current.CreatedAt
	updated.Disabled = current.Disabled
	updated.SeriesID, updated.SeriesNo, updated.SeriesTotal = current.SeriesID, current.SeriesNo, current.SeriesTotal
	apply(&updated.Disabled, in.Disabled)
	if err := s.repo.Update(ctx, updated); err != nil {
		return Expense{}, fmt.Errorf("update expense: %w", err)
	}
	return updated, nil
}

// Delete removes an expense of the period. Expenses created by exchanges can't be
// deleted on their own: delete the exchange instead.
func (s *Service) Delete(ctx context.Context, periodStr, id string) error {
	if _, err := s.editable(ctx, periodStr, id); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id)
}

// editable returns the expense if it belongs to the period and is not managed by an exchange.
func (s *Service) editable(ctx context.Context, periodStr, id string) (Expense, error) {
	p, err := period.Parse(periodStr)
	if err != nil {
		return Expense{}, invalid(err)
	}
	e, err := s.repo.Get(ctx, id)
	if err != nil {
		return Expense{}, err
	}
	if e.Period != p {
		return Expense{}, fmt.Errorf("%w: %q in %s", ErrNotFound, id, p)
	}
	if e.ExchangeID != "" {
		return Expense{}, fmt.Errorf("%w: delete the exchange instead", ErrManagedByExchange)
	}
	return e, nil
}

// apply overwrites dst with *v when v is set.
func apply[T any](dst *T, v *T) {
	if v != nil {
		*dst = *v
	}
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
	Status     string
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
	Status      string
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
			Status:      cmp.Or(item.Status, in.Status),
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
		if err := s.currencies.Require(ctx, e.Amount.Currency()); err != nil {
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
	st, err := status.Parse(in.Status)
	if err != nil {
		return Expense{}, invalid(err)
	}
	return Expense{
		Status:      st,
		ID:          s.newID(),
		Period:      p,
		CategoryID:  strings.TrimSpace(in.CategoryID),
		Amount:      amount,
		Description: description,
		Tags:        tags,
		SpentOn:     spentOn,
		AccountID:   strings.TrimSpace(in.AccountID),
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

// chargeAccount checks the expense's account (it must exist and use the expense's currency)

func (s *Service) chargeAccount(ctx context.Context, e *Expense) error {
	if e.AccountID == "" {
		return nil
	}
	a, err := s.accounts.Get(ctx, e.AccountID)
	if errors.Is(err, account.ErrNotFound) {
		return invalid(err)
	}
	if err != nil {
		return fmt.Errorf("check account: %w", err)
	}
	if a.Amount.Currency() != e.Amount.Currency() {
		return invalid(fmt.Errorf("account %s is in %s, the expense in %s", a.Name, a.Amount.Currency(), e.Amount.Currency()))
	}
	return nil
}

// Filter narrows a listing; empty fields don't filter.
type Filter struct {
	CategoryID string
	Status     string // "planned" or "confirmed"; empty lists both
	Tag        string // compared after normalization, so "#Dulces" matches "dulces"
}

// List returns the expenses of a period ordered by date, optionally filtered.
func (s *Service) List(ctx context.Context, periodStr string, f Filter) ([]Expense, error) {
	p, err := period.Parse(periodStr)
	if err != nil {
		return nil, invalid(err)
	}
	var st status.Status
	if f.Status != "" {
		if st, err = status.Parse(f.Status); err != nil {
			return nil, invalid(err)
		}
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
			(tag != "" && !slices.Contains(e.Tags, tag)) ||
			(st != "" && e.Status != st)
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
