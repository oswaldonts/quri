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
