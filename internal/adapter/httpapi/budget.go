package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/budget"
)

// BudgetService is what the HTTP layer needs from the budget use cases.
type BudgetService interface {
	Set(ctx context.Context, in budget.SetInput) (budget.Budget, error)
	List(ctx context.Context, period string) ([]budget.Budget, error)
	Delete(ctx context.Context, period, categoryID, currency string) error
}

// BudgetParams identify the budget of a category in a period and currency.
type BudgetParams struct {
	PeriodParam
	CategoryID string       `path:"category_id" example:"market"`
	Currency   CurrencyCode `path:"currency"`
}

// SetBudgetInput is the request of PUT /periods/{period}/budgets/{category_id}/{currency}.
type SetBudgetInput struct {
	BudgetParams
	Body struct {
		Amount string `json:"amount" pattern:"^[0-9]{1,13}(\\.[0-9]{1,2})?$" example:"350" doc:"Planned amount, greater than zero."`
	}
}

// Budget is a budget as returned by the API.
type Budget struct {
	Period     string    `json:"period" example:"2026-08"`
	CategoryID string    `json:"category_id" example:"market"`
	Currency   string    `json:"currency" example:"PEN"`
	Amount     string    `json:"amount" example:"350.00"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func toBudget(b budget.Budget) Budget {
	return Budget{
		Period:     b.Period.String(),
		CategoryID: b.CategoryID,
		Currency:   string(b.Amount.Currency()),
		Amount:     b.Amount.String(),
		UpdatedAt:  b.UpdatedAt,
	}
}

// BudgetOutput wraps a Budget as the response body.
type BudgetOutput struct {
	Body Budget
}

// BudgetListOutput is the response of GET /periods/{period}/budgets.
type BudgetListOutput struct {
	Body []Budget
}

func registerBudgets(api huma.API, budgets BudgetService) {
	huma.Register(api, huma.Operation{
		OperationID: "set-budget",
		Method:      http.MethodPut,
		Path:        "/periods/{period}/budgets/{category_id}/{currency}",
		Summary:     "Set the budget of a category",
		Description: "Creates or replaces the planned amount of a category in a period and currency. " +
			"The category must be used in the period. The summary compares it with what was spent.",
		Tags:   []string{tagBudgets},
		Errors: []int{http.StatusUnprocessableEntity},
	}, func(ctx context.Context, in *SetBudgetInput) (*BudgetOutput, error) {
		b, err := budgets.Set(ctx, budget.SetInput{
			Period:     in.Period,
			CategoryID: in.CategoryID,
			Currency:   string(in.Currency),
			Amount:     in.Body.Amount,
		})
		if err != nil {
			return nil, toHTTPError(ctx, "set budget", err)
		}
		return &BudgetOutput{Body: toBudget(b)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-budgets",
		Method:      http.MethodGet,
		Path:        "/periods/{period}/budgets",
		Summary:     "List the budgets of a period",
		Tags:        []string{tagBudgets},
		Errors:      []int{http.StatusUnprocessableEntity},
	}, func(ctx context.Context, in *PeriodParam) (*BudgetListOutput, error) {
		bs, err := budgets.List(ctx, in.Period)
		if err != nil {
			return nil, toHTTPError(ctx, "list budgets", err)
		}
		out := &BudgetListOutput{Body: make([]Budget, 0, len(bs))}
		for _, b := range bs {
			out.Body = append(out.Body, toBudget(b))
		}
		return out, nil
	})

	huma.Register(api, deleteOperation("delete-budget", "/periods/{period}/budgets/{category_id}/{currency}",
		"Delete the budget of a category", "", tagBudgets),
		func(ctx context.Context, in *BudgetParams) (*struct{}, error) {
			if err := budgets.Delete(ctx, in.Period, in.CategoryID, string(in.Currency)); err != nil {
				return nil, toHTTPError(ctx, "delete budget", err)
			}
			return nil, nil
		})
}
