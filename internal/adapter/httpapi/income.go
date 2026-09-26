package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/income"
)

// IncomeRegistrar is what the HTTP layer needs from the income use cases.
// Declaring it here (on the consumer side) lets handler tests use a fake.
type IncomeRegistrar interface {
	Register(ctx context.Context, in income.RegisterInput) (income.Income, error)
}

// RegisterIncomeInput is the request of POST /periods/{period}/incomes.
type RegisterIncomeInput struct {
	PeriodParam
	Body struct {
		Currency    string `json:"currency" enum:"USD,PEN" doc:"ISO 4217 currency code."`
		Amount      string `json:"amount" pattern:"^[0-9]{1,13}(\\.[0-9]{1,2})?$" example:"1330.00" doc:"Decimal string with up to 2 decimal places."`
		Kind        string `json:"kind" enum:"salary,extra" doc:"Exchange incomes are created by POST /periods/{period}/exchanges."`
		Description string `json:"description,omitempty" maxLength:"200" example:"Sueldo julio"`
		ReceivedOn  string `json:"received_on" format:"date" example:"2026-07-31"`
	}
}

// Income is an income as returned by the API.
type Income struct {
	ID          string    `json:"id"`
	Period      string    `json:"period" example:"2026-07"`
	SpendMonth  string    `json:"spend_month" example:"2026-08" doc:"Month in which this money is spent."`
	Currency    string    `json:"currency" example:"USD"`
	Amount      string    `json:"amount" example:"1330.00"`
	Kind        string    `json:"kind" enum:"salary,extra,exchange"`
	Description string    `json:"description"`
	ReceivedOn  string    `json:"received_on" format:"date" example:"2026-07-31"`
	CreatedAt   time.Time `json:"created_at"`
}

// IncomeOutput wraps an Income as the response body.
type IncomeOutput struct {
	Body Income
}

func toIncome(in income.Income) Income {
	return Income{
		ID:          in.ID,
		Period:      in.Period.String(),
		SpendMonth:  in.Period.SpendMonth().String(),
		Currency:    string(in.Amount.Currency()),
		Amount:      in.Amount.String(),
		Kind:        string(in.Kind),
		Description: in.Description,
		ReceivedOn:  in.ReceivedOn.Format(time.DateOnly),
		CreatedAt:   in.CreatedAt,
	}
}

func registerIncomes(api huma.API, incomes IncomeRegistrar) {
	huma.Register(api, huma.Operation{
		OperationID:   "register-income",
		Method:        http.MethodPost,
		Path:          "/periods/{period}/incomes",
		Summary:       "Register an income",
		Description:   "Registers money received for the period (salary or extra).",
		Tags:          []string{tagIncomes},
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusUnprocessableEntity},
	}, func(ctx context.Context, in *RegisterIncomeInput) (*IncomeOutput, error) {
		inc, err := incomes.Register(ctx, income.RegisterInput{
			Period:      in.Period,
			Currency:    in.Body.Currency,
			Amount:      in.Body.Amount,
			Kind:        in.Body.Kind,
			Description: in.Body.Description,
			ReceivedOn:  in.Body.ReceivedOn,
		})
		if err != nil {
			return nil, toHTTPError(ctx, "register income", err)
		}
		return &IncomeOutput{Body: toIncome(inc)}, nil
	})
}
