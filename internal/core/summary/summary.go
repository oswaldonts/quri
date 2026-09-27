// Package summary builds the overview of a period from its detailed entries:
// incomes, expenses (by category, group and tag), the available money and how it
// compares with the real balance.
package summary

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/balance"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/budget"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/category"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/expense"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/income"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/money"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/period"
)

// ErrInvalidInput wraps every validation error.
var ErrInvalidInput = errors.New("invalid input")

// maxTopItems is how many descriptions the "most spent on" ranking shows.
const maxTopItems = 10

// Ports: the summary only reads. The existing repositories already satisfy them.
type (
	IncomeReader interface {
		ListByPeriod(ctx context.Context, p period.Period) ([]income.Income, error)
	}
	ExpenseReader interface {
		ListByPeriod(ctx context.Context, p period.Period) ([]expense.Expense, error)
	}
	BalanceReader interface {
		ListByPeriod(ctx context.Context, p period.Period) ([]balance.Balance, error)
	}
	CategoryReader interface {
		List(ctx context.Context, includeArchived bool) ([]category.Category, error)
	}
	BudgetReader interface {
		ListByPeriod(ctx context.Context, p period.Period) ([]budget.Budget, error)
	}
)

// Summary is the overview of a period, one block per currency with activity.
type Summary struct {
	Period     period.Period
	Currencies []CurrencySummary
}

// CurrencySummary is the overview of one currency.
type CurrencySummary struct {
	Currency money.Currency
	Income   IncomeTotals

	Contributions []ContributionTotal // extras assigned to categories, highest first
	Categories    []CategoryTotal     // categories with expenses or a budget, by amount spent
	Groups        []GroupTotal        // by amount spent, highest first
	Tags          []TagTotal          // by amount spent, highest first
	TopItems      []ItemTotal         // descriptions with the most money spent (exchanges excluded)

	Committed     money.Money  // spent in committed categories (must be paid every month)
	Discretionary money.Money  // spent in the other non-system categories
	Exchanged     money.Money  // spent in the system exchange category
	Spent         money.Money  // total spent
	Budgeted      *money.Money // sum of the category budgets, if any

	CarriedOver money.Money // brought from the previous period (see CarriedFrom)
	CarriedFrom CarrySource // "" when nothing is carried

	Available  money.Money  // carried over + income total − spent: what should be left
	Real       *money.Money // real balance, if registered
	Difference *money.Money // available − real; 0 means every entry is registered
}

// IncomeTotals splits the income of a currency by kind.
type IncomeTotals struct {
	Salary   money.Money
	Exchange money.Money
	Extra    money.Money // includes contributions to categories
	Total    money.Money
}

// CategoryTotal is what was spent in a category.
type CategoryTotal struct {
	Category  category.Category
	Spent     money.Money
	Count     int          // number of expenses
	Budget    *money.Money // planned amount, if any
	Remaining *money.Money // budget − spent; negative when over budget
}

// ContributionTotal is the extra money assigned to a category (an "aporte").
type ContributionTotal struct {
	Category category.Category
	Amount   money.Money
	Count    int // number of contributions
}

// GroupTotal is the money spent in a group of categories.
type GroupTotal struct {
	Group category.Group
	Spent money.Money
}

// TagTotal is the money spent on expenses with a tag. An expense with several
// tags counts in each of them, so tag totals don't add up to Spent.
type TagTotal struct {
	Tag   string
	Spent money.Money
	Count int
}

// ItemTotal is the money spent on expenses with the same description
// (compared case-insensitively).
type ItemTotal struct {
	Description string
	Spent       money.Money
	Count       int
}

// Service builds summaries.
type Service struct {
	incomes    IncomeReader
	expenses   ExpenseReader
	balances   BalanceReader
	categories CategoryReader
	budgets    BudgetReader
	bounds     PeriodBounds
}

// NewService returns a Service reading from the given repositories.
func NewService(incomes IncomeReader, expenses ExpenseReader, balances BalanceReader,
	categories CategoryReader, budgets BudgetReader, bounds PeriodBounds) *Service {
	return &Service{
		incomes: incomes, expenses: expenses, balances: balances,
		categories: categories, budgets: budgets, bounds: bounds,
	}
}

// Get builds the summary of a period, including what it carries from the previous
// one, as seen from the view ("confirmed" by default, or "projected").
func (s *Service) Get(ctx context.Context, periodStr, viewStr string) (Summary, error) {
	p, err := period.Parse(periodStr)
	if err != nil {
		return Summary{}, fmt.Errorf("%w: %w", ErrInvalidInput, err)
	}
	view, err := ParseView(viewStr)
	if err != nil {
		return Summary{}, err
	}
	carry, err := s.carryInto(ctx, p, view)
	if err != nil {
		return Summary{}, err
	}
	return s.summarize(ctx, p, carry, view)
}

// summarize builds the summary of a period given what it carries.
func (s *Service) summarize(ctx context.Context, p period.Period, carry carries, view View) (Summary, error) {
	incomes, expenses, balances, err := s.movements(ctx, p, view)
	if err != nil {
		return Summary{}, err
	}
	catalog, err := s.categories.List(ctx, true) // archived too: old periods may use them
	if err != nil {
		return Summary{}, fmt.Errorf("list categories: %w", err)
	}
	budgets, err := s.budgets.ListByPeriod(ctx, p)
	if err != nil {
		return Summary{}, fmt.Errorf("list budgets: %w", err)
	}
	return build(p, entries{incomes, expenses, balances, budgets}, catalog, carry), nil
}

// GetCurrency builds the summary of one currency in a period. A currency without
// activity returns an empty block (every amount in zero) instead of an error.
func (s *Service) GetCurrency(ctx context.Context, periodStr, currencyStr, view string) (period.Period, CurrencySummary, error) {
	currency, err := money.ParseCurrency(currencyStr)
	if err != nil {
		return period.Period{}, CurrencySummary{}, fmt.Errorf("%w: %w", ErrInvalidInput, err)
	}
	sum, err := s.Get(ctx, periodStr, view)
	if err != nil {
		return period.Period{}, CurrencySummary{}, err
	}
	return sum.Period, currencyOf(sum, currency), nil
}

// entries groups what was registered in a period.
type entries struct {
	incomes  []income.Income
	expenses []expense.Expense
	balances []balance.Balance
	budgets  []budget.Budget
}

// build is pure: it only computes, which keeps it easy to test.
func build(p period.Period, e entries, catalog []category.Category, carry carries) Summary {

	categories := make(map[string]category.Category, len(catalog))
	for _, c := range catalog {
		categories[c.ID] = c
	}

	builders := map[money.Currency]*builder{}
	get := func(c money.Currency) *builder {
		if builders[c] == nil {
			builders[c] = newBuilder(c, categories)
		}
		return builders[c]
	}

	for c, cr := range carry {
		get(c).carry = cr
	}
	for _, in := range e.incomes {
		get(in.Amount.Currency()).addIncome(in)
	}
	for _, ex := range e.expenses {
		get(ex.Amount.Currency()).addExpense(ex)
	}
	for _, b := range e.balances {
		get(b.Amount.Currency()).setReal(b.Amount)
	}
	for _, b := range e.budgets {
		get(b.Amount.Currency()).setBudget(b)
	}

	out := Summary{Period: p}
	for _, c := range slices.Sorted(maps.Keys(builders)) {
		out.Currencies = append(out.Currencies, builders[c].result())
	}
	return out
}

// builder accumulates the totals of one currency.
type builder struct {
	currency   money.Currency
	categories map[string]category.Category

	income         IncomeTotals
	byCategory     map[string]*CategoryTotal
	byContribution map[string]*ContributionTotal
	byGroup        map[category.Group]money.Money
	byTag          map[string]*TagTotal
	byItem         map[string]*ItemTotal

	committed, discretionary, exchanged, spent money.Money
	carry                                      Carry
	real                                       *money.Money
}

func newBuilder(c money.Currency, categories map[string]category.Category) *builder {
	zero := money.Zero(c)
	return &builder{
		currency:       c,
		categories:     categories,
		income:         IncomeTotals{Salary: zero, Exchange: zero, Extra: zero, Total: zero},
		byCategory:     map[string]*CategoryTotal{},
		byContribution: map[string]*ContributionTotal{},
		byGroup:        map[category.Group]money.Money{},
		byTag:          map[string]*TagTotal{},
		byItem:         map[string]*ItemTotal{},
		committed:      zero,
		discretionary:  zero,
		exchanged:      zero,
		carry:          Carry{Amount: zero},
		spent:          zero,
	}
}

func (b *builder) addIncome(in income.Income) {
	switch in.Kind {
	case income.KindSalary:
		b.income.Salary = add(b.income.Salary, in.Amount)
	case income.KindExchange:
		b.income.Exchange = add(b.income.Exchange, in.Amount)
	case income.KindExtra:
		b.income.Extra = add(b.income.Extra, in.Amount)
		if in.CategoryID != "" {
			ct := b.byContribution[in.CategoryID]
			if ct == nil {
				ct = &ContributionTotal{Category: b.lookup(in.CategoryID), Amount: money.Zero(b.currency)}
				b.byContribution[in.CategoryID] = ct
			}
			ct.Amount = add(ct.Amount, in.Amount)
			ct.Count++
		}
	}
	b.income.Total = add(b.income.Total, in.Amount)
}

func (b *builder) addExpense(e expense.Expense) {
	ct := b.category(e.CategoryID)
	ct.Spent = add(ct.Spent, e.Amount)
	ct.Count++

	group := ct.Category.Group
	groupTotal, ok := b.byGroup[group]
	if !ok {
		groupTotal = money.Zero(b.currency)
	}
	b.byGroup[group] = add(groupTotal, e.Amount)

	switch {
	case ct.Category.IsSystem():
		b.exchanged = add(b.exchanged, e.Amount)
	case ct.Category.Committed:
		b.committed = add(b.committed, e.Amount)
	default:
		b.discretionary = add(b.discretionary, e.Amount)
	}
	b.spent = add(b.spent, e.Amount)

	for _, tag := range e.Tags {
		t := b.byTag[tag]
		if t == nil {
			t = &TagTotal{Tag: tag, Spent: money.Zero(b.currency)}
			b.byTag[tag] = t
		}
		t.Spent = add(t.Spent, e.Amount)
		t.Count++
	}

	// Exchanges are transfers between currencies, not purchases: keep them out of the ranking.
	if e.ExchangeID == "" && e.Description != "" {
		key := strings.ToLower(strings.Join(strings.Fields(e.Description), " "))
		it := b.byItem[key]
		if it == nil {
			it = &ItemTotal{Description: e.Description, Spent: money.Zero(b.currency)}
			b.byItem[key] = it
		}
		it.Spent = add(it.Spent, e.Amount)
		it.Count++
	}
}

func (b *builder) setReal(m money.Money) {
	b.real = &m
}

func (b *builder) setBudget(bud budget.Budget) {
	amount := bud.Amount
	b.category(bud.CategoryID).Budget = &amount
}

// category returns the running total of a category, creating it on first use.
func (b *builder) category(id string) *CategoryTotal {
	ct := b.byCategory[id]
	if ct == nil {
		ct = &CategoryTotal{Category: b.lookup(id), Spent: money.Zero(b.currency)}
		b.byCategory[id] = ct
	}
	return ct
}

// lookup returns the category from the catalog, or a placeholder if it is unknown.
func (b *builder) lookup(id string) category.Category {
	if c, ok := b.categories[id]; ok {
		return c
	}
	return category.Category{ID: id, Name: id, Group: category.GroupOther}
}

func (b *builder) result() CurrencySummary {
	cs := CurrencySummary{
		Currency:      b.currency,
		Income:        b.income,
		Committed:     b.committed,
		Discretionary: b.discretionary,
		Exchanged:     b.exchanged,
		Spent:         b.spent,
		CarriedOver:   b.carry.Amount,
		CarriedFrom:   b.carry.From,
		Available:     sub(add(b.carry.Amount, b.income.Total), b.spent),
		Real:          b.real,
	}
	if b.real != nil {
		diff := sub(cs.Available, *b.real)
		cs.Difference = &diff
	}

	for _, ct := range b.byCategory {
		if ct.Budget != nil {
			remaining := sub(*ct.Budget, ct.Spent)
			ct.Remaining = &remaining
			total := *ct.Budget
			if cs.Budgeted != nil {
				total = add(*cs.Budgeted, total)
			}
			cs.Budgeted = &total
		}
		cs.Categories = append(cs.Categories, *ct)
	}
	slices.SortFunc(cs.Categories, func(x, y CategoryTotal) int {
		return cmp.Or(cmp.Compare(y.Spent.Amount(), x.Spent.Amount()), cmp.Compare(x.Category.Name, y.Category.Name))
	})

	for _, ct := range b.byContribution {
		cs.Contributions = append(cs.Contributions, *ct)
	}
	slices.SortFunc(cs.Contributions, func(x, y ContributionTotal) int {
		return cmp.Or(cmp.Compare(y.Amount.Amount(), x.Amount.Amount()), cmp.Compare(x.Category.Name, y.Category.Name))
	})

	for g, spent := range b.byGroup {
		cs.Groups = append(cs.Groups, GroupTotal{Group: g, Spent: spent})
	}
	slices.SortFunc(cs.Groups, func(x, y GroupTotal) int {
		return cmp.Or(cmp.Compare(y.Spent.Amount(), x.Spent.Amount()), cmp.Compare(x.Group, y.Group))
	})

	for _, t := range b.byTag {
		cs.Tags = append(cs.Tags, *t)
	}
	slices.SortFunc(cs.Tags, func(x, y TagTotal) int {
		return cmp.Or(cmp.Compare(y.Spent.Amount(), x.Spent.Amount()), cmp.Compare(x.Tag, y.Tag))
	})

	for _, it := range b.byItem {
		cs.TopItems = append(cs.TopItems, *it)
	}
	slices.SortFunc(cs.TopItems, func(x, y ItemTotal) int {
		return cmp.Or(cmp.Compare(y.Spent.Amount(), x.Spent.Amount()), cmp.Compare(x.Description, y.Description))
	})
	if len(cs.TopItems) > maxTopItems {
		cs.TopItems = cs.TopItems[:maxTopItems]
	}
	return cs
}

// add and sub only receive amounts of the builder's currency, so a mismatch
// would be a programming error.
func add(a, b money.Money) money.Money {
	sum, err := a.Add(b)
	if err != nil {
		panic(err)
	}
	return sum
}

func sub(a, b money.Money) money.Money {
	diff, err := a.Sub(b)
	if err != nil {
		panic(err)
	}
	return diff
}
