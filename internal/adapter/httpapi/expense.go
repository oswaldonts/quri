package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/expense"
)

// ExpenseService is what the HTTP layer needs from the expense use cases.
type ExpenseService interface {
	Register(ctx context.Context, in expense.RegisterInput) (expense.Expense, error)
	List(ctx context.Context, period string) ([]expense.Expense, error)
}

// RegisterExpenseInput is the request of POST /periods/{period}/expenses.
type RegisterExpenseInput struct {
	PeriodParam
	Body struct {
		CategoryID  string `json:"category_id" example:"market" doc:"Must be used in the period (see GET /periods/{period}/categories)."`
		Currency    string `json:"currency" enum:"USD,PEN" doc:"ISO 4217 currency code."`
		Amount      string `json:"amount" pattern:"^[0-9]{1,13}(\\.[0-9]{1,2})?$" example:"45.50" doc:"Decimal string with up to 2 decimal places."`
		Description string `json:"description,omitempty" maxLength:"200" example:"Mercado semanal"`
		SpentOn     string `json:"spent_on" format:"date" example:"2026-08-03"`
	}
}

// Expense is an expense as returned by the API.
type Expense struct {
	ID          string    `json:"id"`
	Period      string    `json:"period" example:"2026-07"`
	CategoryID  string    `json:"category_id" example:"market"`
	Currency    string    `json:"currency" example:"PEN"`
	Amount      string    `json:"amount" example:"45.50"`
	Description string    `json:"description"`
	SpentOn     string    `json:"spent_on" format:"date" example:"2026-08-03"`
	ExchangeID  string    `json:"exchange_id,omitempty" doc:"Set when the expense is the source side of an exchange."`
	CreatedAt   time.Time `json:"created_at"`
}

func toExpense(e expense.Expense) Expense {
	return Expense{
		ID:          e.ID,
		Period:      e.Period.String(),
		CategoryID:  e.CategoryID,
		Currency:    string(e.Amount.Currency()),
		Amount:      e.Amount.String(),
		Description: e.Description,
		SpentOn:     e.SpentOn.Format(time.DateOnly),
		ExchangeID:  e.ExchangeID,
		CreatedAt:   e.CreatedAt,
	}
}

// ExpenseOutput wraps an Expense as the response body.
type ExpenseOutput struct {
	Body Expense
}

// ExpenseListOutput is the response of GET /periods/{period}/expenses.
type ExpenseListOutput struct {
	Body []Expense
}

func registerExpenses(api huma.API, expenses ExpenseService) {
	huma.Register(api, huma.Operation{
		OperationID: "register-expense",
		Method:      http.MethodPost,
		Path:        "/periods/{period}/expenses",
		Summary:     "Register an expense",
		Description: "The category must be used in the period. The first expense of a period fixes its category list. " +
			"Expenses in the system category \"exchange\" are created by POST /periods/{period}/exchanges.",
		Tags:          []string{tagExpenses},
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusUnprocessableEntity},
	}, func(ctx context.Context, in *RegisterExpenseInput) (*ExpenseOutput, error) {
		e, err := expenses.Register(ctx, expense.RegisterInput{
			Period:      in.Period,
			CategoryID:  in.Body.CategoryID,
			Currency:    in.Body.Currency,
			Amount:      in.Body.Amount,
			Description: in.Body.Description,
			SpentOn:     in.Body.SpentOn,
		})
		if err != nil {
			return nil, toHTTPError(ctx, "register expense", err)
		}
		return &ExpenseOutput{Body: toExpense(e)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-expenses",
		Method:      http.MethodGet,
		Path:        "/periods/{period}/expenses",
		Summary:     "List the expenses of a period",
		Description: "Ordered by date, including the ones created by exchanges.",
		Tags:        []string{tagExpenses},
		Errors:      []int{http.StatusUnprocessableEntity},
	}, func(ctx context.Context, in *PeriodParam) (*ExpenseListOutput, error) {
		es, err := expenses.List(ctx, in.Period)
		if err != nil {
			return nil, toHTTPError(ctx, "list expenses", err)
		}
		out := &ExpenseListOutput{Body: make([]Expense, 0, len(es))}
		for _, e := range es {
			out.Body = append(out.Body, toExpense(e))
		}
		return out, nil
	})
}
