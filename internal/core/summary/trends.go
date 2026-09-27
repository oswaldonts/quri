package summary

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/category"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/money"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/period"
)

// maxTrendMonths bounds the range of a trends query.
const maxTrendMonths = 24

// TrendsInput selects the range and currency of a trends query.
type TrendsInput struct {
	From     string // YYYY-MM, included
	To       string // YYYY-MM, included
	Currency string // ISO 4217 code
	View     string // "confirmed" (default) or "projected"
}

// Trends compares several consecutive periods in one currency.
type Trends struct {
	From, To   period.Period
	Currency   money.Currency
	Months     int
	Periods    []PeriodTrend   // one per month, in order (empty months included)
	Categories []CategoryTrend // by total spent, highest first
	Tags       []TagTrend      // by total spent, highest first
}

// PeriodTrend is the headline of one month.
type PeriodTrend struct {
	Period    period.Period
	Income    money.Money
	Spent     money.Money
	Available money.Money
}

// CategoryTrend is the spending of a category across the range.
type CategoryTrend struct {
	Category category.Category
	Total    money.Money
	Average  money.Money   // Total / Months, truncated
	Series   []money.Money // spent per month, aligned with Trends.Periods
}

// TagTrend is the spending with a tag across the range.
type TagTrend struct {
	Tag     string
	Total   money.Money
	Average money.Money
	Series  []money.Money
}

// Trends builds the month-by-month comparison of a currency.
func (s *Service) Trends(ctx context.Context, in TrendsInput) (Trends, error) {
	from, err := period.Parse(in.From)
	if err != nil {
		return Trends{}, fmt.Errorf("%w: from: %w", ErrInvalidInput, err)
	}
	to, err := period.Parse(in.To)
	if err != nil {
		return Trends{}, fmt.Errorf("%w: to: %w", ErrInvalidInput, err)
	}
	currency, err := money.ParseCurrency(in.Currency)
	if err != nil {
		return Trends{}, fmt.Errorf("%w: %w", ErrInvalidInput, err)
	}
	view, err := ParseView(in.View)
	if err != nil {
		return Trends{}, err
	}
	months := from.MonthsBetween(to)
	if months == 0 {
		return Trends{}, fmt.Errorf("%w: %w", ErrInvalidInput, errors.New("from must not be after to"))
	}
	if months > maxTrendMonths {
		return Trends{}, fmt.Errorf("%w: at most %d months, got %d", ErrInvalidInput, maxTrendMonths, months)
	}

	// The carry is computed once for the first month; each month then passes its
	// leftover to the next one.
	carry, err := s.carryInto(ctx, from, view)
	if err != nil {
		return Trends{}, err
	}
	perMonth := make([]CurrencySummary, 0, months)
	for p := from; p.Compare(to) <= 0; p = p.Next() {
		sum, err := s.summarize(ctx, p, carry, view)
		if err != nil {
			return Trends{}, err
		}
		perMonth = append(perMonth, currencyOf(sum, currency))
		carry = carriesFromSummary(sum)
	}
	return buildTrends(from, to, currency, perMonth), nil
}

// currencyOf returns the block of the currency, or an empty one if it had no activity.
func currencyOf(s Summary, c money.Currency) CurrencySummary {
	for _, cs := range s.Currencies {
		if cs.Currency == c {
			return cs
		}
	}
	return newBuilder(c, nil).result()
}

// buildTrends is pure: it only combines the monthly summaries.
func buildTrends(from, to period.Period, c money.Currency, perMonth []CurrencySummary) Trends {
	n := len(perMonth)
	t := Trends{From: from, To: to, Currency: c, Months: n}

	categories := map[string]*CategoryTrend{}
	tags := map[string]*TagTrend{}
	zeros := func() []money.Money {
		s := make([]money.Money, n)
		for i := range s {
			s[i] = money.Zero(c)
		}
		return s
	}

	p := from
	for i, cs := range perMonth {
		t.Periods = append(t.Periods, PeriodTrend{Period: p, Income: cs.Income.Total, Spent: cs.Spent, Available: cs.Available})
		p = p.Next()

		for _, ct := range cs.Categories {
			tr := categories[ct.Category.ID]
			if tr == nil {
				tr = &CategoryTrend{Category: ct.Category, Series: zeros()}
				categories[ct.Category.ID] = tr
			}
			tr.Series[i] = ct.Spent
		}
		for _, tg := range cs.Tags {
			tr := tags[tg.Tag]
			if tr == nil {
				tr = &TagTrend{Tag: tg.Tag, Series: zeros()}
				tags[tg.Tag] = tr
			}
			tr.Series[i] = tg.Spent
		}
	}

	for _, tr := range categories {
		tr.Total, tr.Average = totalAndAverage(c, tr.Series)
		t.Categories = append(t.Categories, *tr)
	}
	slices.SortFunc(t.Categories, func(x, y CategoryTrend) int {
		return cmp.Or(cmp.Compare(y.Total.Amount(), x.Total.Amount()), cmp.Compare(x.Category.Name, y.Category.Name))
	})

	for _, tr := range tags {
		tr.Total, tr.Average = totalAndAverage(c, tr.Series)
		t.Tags = append(t.Tags, *tr)
	}
	slices.SortFunc(t.Tags, func(x, y TagTrend) int {
		return cmp.Or(cmp.Compare(y.Total.Amount(), x.Total.Amount()), cmp.Compare(x.Tag, y.Tag))
	})
	return t
}

func totalAndAverage(c money.Currency, series []money.Money) (money.Money, money.Money) {
	total := money.Zero(c)
	for _, m := range series {
		total = add(total, m)
	}
	// Integer division on minor units truncates, like every other conversion.
	average, err := money.New(total.Amount()/int64(len(series)), c)
	if err != nil {
		panic(err) // c comes from a valid Money
	}
	return total, average
}
