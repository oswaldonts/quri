package httpapi

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/money"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/summary"
)

// SummaryService is what the HTTP layer needs to build period summaries.
type SummaryService interface {
	Get(ctx context.Context, period string) (summary.Summary, error)
	Trends(ctx context.Context, in summary.TrendsInput) (summary.Trends, error)
	Available(ctx context.Context, period string) ([]summary.Available, error)
}

// Summary is the overview of a period.
type Summary struct {
	Period     string            `json:"period" example:"2026-07"`
	SpendMonth string            `json:"spend_month" example:"2026-08"`
	Currencies []CurrencySummary `json:"currencies" doc:"One block per currency with incomes, expenses or a real balance."`
}

// CurrencySummary is the overview of one currency.
type CurrencySummary struct {
	Currency string `json:"currency" example:"PEN"`
	Income   struct {
		Salary   string `json:"salary"`
		Exchange string `json:"exchange" doc:"Received through currency exchanges."`
		Extra    string `json:"extra" doc:"Includes contributions to categories."`
		Total    string `json:"total"`
	} `json:"income"`
	Categories    []CategoryTotal `json:"categories" doc:"Categories with activity, by amount spent (highest first)."`
	Groups        []GroupTotal    `json:"groups" doc:"By amount spent (highest first)."`
	Tags          []TagTotal      `json:"tags" doc:"By amount spent. An expense with several tags counts in each one."`
	TopItems      []ItemTotal     `json:"top_items" doc:"Up to 10 descriptions with the most money spent (case-insensitive; exchanges excluded)."`
	Committed     string          `json:"committed" doc:"Spent in committed categories (must be paid every month)."`
	Discretionary string          `json:"discretionary" doc:"Spent in the other non-system categories."`
	Exchanged     string          `json:"exchanged" doc:"Spent in the system category Cambio."`
	Spent         string          `json:"spent"`
	Budgeted      string          `json:"budgeted,omitempty" doc:"Sum of the category budgets, if any."`
	Available     string          `json:"available" doc:"Income total minus spent: the money that should be left."`
	Real          string          `json:"real,omitempty" doc:"Real balance, if registered."`
	Difference    string          `json:"difference,omitempty" doc:"available − real. 0 means every entry is registered; negative: more money than registered; positive: an expense is missing."`
}

// CategoryTotal is what a category received and spent in a currency.
type CategoryTotal struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Group       string `json:"group"`
	Committed   bool   `json:"committed"`
	Contributed string `json:"contributed" doc:"Extra incomes contributed to the category."`
	Spent       string `json:"spent"`
	Count       int    `json:"count" doc:"Number of expenses."`
	Budget      string `json:"budget,omitempty" doc:"Planned amount, if the category has a budget."`
	Remaining   string `json:"remaining,omitempty" doc:"budget − spent; negative when over budget."`
}

// GroupTotal is the money spent in a group.
type GroupTotal struct {
	Group string `json:"group"`
	Spent string `json:"spent"`
}

// TagTotal is the money spent with a tag.
type TagTotal struct {
	Tag   string `json:"tag"`
	Spent string `json:"spent"`
	Count int    `json:"count"`
}

// ItemTotal is the money spent on a description.
type ItemTotal struct {
	Description string `json:"description"`
	Spent       string `json:"spent"`
	Count       int    `json:"count"`
}

// SummaryOutput wraps a Summary as the response body.
type SummaryOutput struct {
	Body Summary
}

func toSummary(s summary.Summary) Summary {
	out := Summary{
		Period:     s.Period.String(),
		SpendMonth: s.Period.SpendMonth().String(),
		Currencies: make([]CurrencySummary, 0, len(s.Currencies)),
	}
	for _, c := range s.Currencies {
		cs := CurrencySummary{
			Currency:      string(c.Currency),
			Categories:    make([]CategoryTotal, 0, len(c.Categories)),
			Groups:        make([]GroupTotal, 0, len(c.Groups)),
			Tags:          make([]TagTotal, 0, len(c.Tags)),
			TopItems:      make([]ItemTotal, 0, len(c.TopItems)),
			Committed:     c.Committed.String(),
			Discretionary: c.Discretionary.String(),
			Exchanged:     c.Exchanged.String(),
			Spent:         c.Spent.String(),
			Budgeted:      optional(c.Budgeted),
			Available:     c.Available.String(),
			Real:          optional(c.Real),
			Difference:    optional(c.Difference),
		}
		cs.Income.Salary = c.Income.Salary.String()
		cs.Income.Exchange = c.Income.Exchange.String()
		cs.Income.Extra = c.Income.Extra.String()
		cs.Income.Total = c.Income.Total.String()

		for _, ct := range c.Categories {
			cs.Categories = append(cs.Categories, CategoryTotal{
				ID:          ct.Category.ID,
				Name:        ct.Category.Name,
				Group:       string(ct.Category.Group),
				Committed:   ct.Category.Committed,
				Contributed: ct.Contributed.String(),
				Spent:       ct.Spent.String(),
				Count:       ct.Count,
				Budget:      optional(ct.Budget),
				Remaining:   optional(ct.Remaining),
			})
		}
		for _, g := range c.Groups {
			cs.Groups = append(cs.Groups, GroupTotal{Group: string(g.Group), Spent: g.Spent.String()})
		}
		for _, t := range c.Tags {
			cs.Tags = append(cs.Tags, TagTotal{Tag: t.Tag, Spent: t.Spent.String(), Count: t.Count})
		}
		for _, it := range c.TopItems {
			cs.TopItems = append(cs.TopItems, ItemTotal{Description: it.Description, Spent: it.Spent.String(), Count: it.Count})
		}
		out.Currencies = append(out.Currencies, cs)
	}
	return out
}

// optional formats a money pointer, "" when nil (omitted from the JSON).
func optional(m *money.Money) string {
	if m == nil {
		return ""
	}
	return m.String()
}

func registerSummary(api huma.API, summaries SummaryService) {
	huma.Register(api, huma.Operation{
		OperationID: "get-summary",
		Method:      http.MethodGet,
		Path:        "/periods/{period}/summary",
		Summary:     "Summary of a period",
		Description: "Per currency: income by kind, spending by category, group and tag, the most bought " +
			"items, the available money (income − spent) and its difference with the real balance.",
		Tags:   []string{tagSummary},
		Errors: []int{http.StatusUnprocessableEntity},
	}, func(ctx context.Context, in *PeriodParam) (*SummaryOutput, error) {
		s, err := summaries.Get(ctx, in.Period)
		if err != nil {
			return nil, toHTTPError(ctx, "get summary", err)
		}
		return &SummaryOutput{Body: toSummary(s)}, nil
	})
}

// TrendsInput is the request of GET /trends.
type TrendsInput struct {
	From     string       `query:"from" required:"true" pattern:"^[0-9]{4}-(0[1-9]|1[0-2])$" example:"2026-01" doc:"First period, included."`
	To       string       `query:"to" required:"true" pattern:"^[0-9]{4}-(0[1-9]|1[0-2])$" example:"2026-06" doc:"Last period, included (at most 24 months)."`
	Currency CurrencyCode `query:"currency" required:"true"`
}

// Trends compares several consecutive periods in one currency.
type Trends struct {
	From       string          `json:"from" example:"2026-01"`
	To         string          `json:"to" example:"2026-06"`
	Currency   string          `json:"currency" example:"PEN"`
	Months     int             `json:"months"`
	Periods    []PeriodTrend   `json:"periods" doc:"One per month in order, including months without activity."`
	Categories []CategoryTrend `json:"categories" doc:"By total spent in the range, highest first."`
	Tags       []TagTrend      `json:"tags" doc:"By total spent in the range, highest first."`
}

// PeriodTrend is the headline of one month.
type PeriodTrend struct {
	Period    string `json:"period"`
	Income    string `json:"income"`
	Spent     string `json:"spent"`
	Available string `json:"available"`
}

// CategoryTrend is the spending of a category across the range.
type CategoryTrend struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Group   string   `json:"group"`
	Total   string   `json:"total"`
	Average string   `json:"average" doc:"Total divided by the number of months, truncated."`
	Series  []string `json:"series" doc:"Spent per month, aligned with periods."`
}

// TagTrend is the spending with a tag across the range.
type TagTrend struct {
	Tag     string   `json:"tag"`
	Total   string   `json:"total"`
	Average string   `json:"average"`
	Series  []string `json:"series"`
}

// TrendsOutput wraps Trends as the response body.
type TrendsOutput struct {
	Body Trends
}

func toStrings(ms []money.Money) []string {
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		out = append(out, m.String())
	}
	return out
}

func toTrends(t summary.Trends) Trends {
	out := Trends{
		From:       t.From.String(),
		To:         t.To.String(),
		Currency:   string(t.Currency),
		Months:     t.Months,
		Periods:    make([]PeriodTrend, 0, len(t.Periods)),
		Categories: make([]CategoryTrend, 0, len(t.Categories)),
		Tags:       make([]TagTrend, 0, len(t.Tags)),
	}
	for _, p := range t.Periods {
		out.Periods = append(out.Periods, PeriodTrend{
			Period: p.Period.String(), Income: p.Income.String(), Spent: p.Spent.String(), Available: p.Available.String(),
		})
	}
	for _, c := range t.Categories {
		out.Categories = append(out.Categories, CategoryTrend{
			ID: c.Category.ID, Name: c.Category.Name, Group: string(c.Category.Group),
			Total: c.Total.String(), Average: c.Average.String(), Series: toStrings(c.Series),
		})
	}
	for _, tg := range t.Tags {
		out.Tags = append(out.Tags, TagTrend{
			Tag: tg.Tag, Total: tg.Total.String(), Average: tg.Average.String(), Series: toStrings(tg.Series),
		})
	}
	return out
}

func registerTrends(api huma.API, summaries SummaryService) {
	huma.Register(api, huma.Operation{
		OperationID: "get-trends",
		Method:      http.MethodGet,
		Path:        "/trends",
		Summary:     "Trends across periods",
		Description: "Month by month in one currency: income, spent and available per period, and per category " +
			"and tag the total, the monthly average and the series (ready to chart).",
		Tags:   []string{tagSummary},
		Errors: []int{http.StatusUnprocessableEntity},
	}, func(ctx context.Context, in *TrendsInput) (*TrendsOutput, error) {
		t, err := summaries.Trends(ctx, summary.TrendsInput{From: in.From, To: in.To, Currency: string(in.Currency)})
		if err != nil {
			return nil, toHTTPError(ctx, "get trends", err)
		}
		return &TrendsOutput{Body: toTrends(t)}, nil
	})
}

// Available is what is left of a period in one currency.
type Available struct {
	Currency  string `json:"currency" example:"PEN"`
	Income    string `json:"income" example:"4018.00" doc:"Every income of the period: salary, exchanges and extras."`
	Spent     string `json:"spent" example:"1000.00" doc:"Every expense of the period, including the exchanges."`
	Available string `json:"available" example:"3018.00" doc:"income − spent: the money left."`
}

// AvailableOutput is the response of GET /periods/{period}/available.
type AvailableOutput struct {
	Body []Available
}

func registerAvailable(api huma.API, summaries SummaryService) {
	huma.Register(api, huma.Operation{
		OperationID: "get-available",
		Method:      http.MethodGet,
		Path:        "/periods/{period}/available",
		Summary:     "Money left in a period",
		Description: "Per currency, income − spent. The quick answer to \"how much do I have left?\"; " +
			"the summary has the same figure plus the detail by category, tag and item.",
		Tags:   []string{tagSummary},
		Errors: []int{http.StatusUnprocessableEntity},
	}, func(ctx context.Context, in *PeriodParam) (*AvailableOutput, error) {
		as, err := summaries.Available(ctx, in.Period)
		if err != nil {
			return nil, toHTTPError(ctx, "get available", err)
		}
		out := &AvailableOutput{Body: make([]Available, 0, len(as))}
		for _, a := range as {
			out.Body = append(out.Body, Available{
				Currency:  string(a.Currency),
				Income:    a.Income.String(),
				Spent:     a.Spent.String(),
				Available: a.Available.String(),
			})
		}
		return out, nil
	})
}
