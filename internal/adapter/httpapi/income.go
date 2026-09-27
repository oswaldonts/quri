package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/income"
)

// IncomeService is what the HTTP layer needs from the income use cases.
// Declaring it here (on the consumer side) lets handler tests use a fake.
type IncomeService interface {
	Register(ctx context.Context, in income.RegisterInput) (income.Income, error)
	List(ctx context.Context, period string) ([]income.Income, error)
	Update(ctx context.Context, period, id string, in income.UpdateInput) (income.Income, error)
	Delete(ctx context.Context, period, id string) error
}

// UpdateIncomeInput is the request of PATCH /periods/{period}/incomes/{id}: only the sent fields change.
type UpdateIncomeInput struct {
	EntryParams
	Body struct {
		Currency    *CurrencyCode `json:"currency,omitempty"`
		Amount      *string       `json:"amount,omitempty" pattern:"^[0-9]{1,13}(\\.[0-9]{1,2})?$"`
		Kind        *string       `json:"kind,omitempty" enum:"salary,extra"`
		Description *string       `json:"description,omitempty" maxLength:"200"`
		ReceivedOn  *string       `json:"received_on,omitempty" format:"date"`
		CategoryID  *string       `json:"category_id,omitempty" doc:"Contribution category; an empty string removes it."`
	}
}

// RegisterIncomeInput is the request of POST /periods/{period}/incomes.
type RegisterIncomeInput struct {
	PeriodParam
	Body struct {
		Currency    CurrencyCode `json:"currency"`
		Amount      string       `json:"amount" pattern:"^[0-9]{1,13}(\\.[0-9]{1,2})?$" example:"1330.00" doc:"Decimal string with up to 2 decimal places."`
		Kind        string       `json:"kind" enum:"salary,extra" doc:"Exchange incomes are created by POST /periods/{period}/exchanges."`
		Description string       `json:"description,omitempty" maxLength:"200" example:"Sueldo julio"`
		ReceivedOn  string       `json:"received_on" format:"date" example:"2026-07-31"`
		CategoryID  string       `json:"category_id,omitempty" example:"loans" doc:"Only for kind extra: the category this money is contributed to (an \"aporte\"). Must be used in the period."`
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
	CategoryID  string    `json:"category_id,omitempty" doc:"Set when an extra income is contributed to a category."`
	Description string    `json:"description"`
	ReceivedOn  string    `json:"received_on" format:"date" example:"2026-07-31"`
	CreatedAt   time.Time `json:"created_at"`
}

// IncomeOutput wraps an Income as the response body.
type IncomeOutput struct {
	Body Income
}

// IncomeListOutput is the response of GET /periods/{period}/incomes.
type IncomeListOutput struct {
	Body []Income
}

func toIncome(in income.Income) Income {
	return Income{
		ID:          in.ID,
		Period:      in.Period.String(),
		SpendMonth:  in.Period.SpendMonth().String(),
		Currency:    string(in.Amount.Currency()),
		Amount:      in.Amount.String(),
		Kind:        string(in.Kind),
		CategoryID:  in.CategoryID,
		Description: in.Description,
		ReceivedOn:  in.ReceivedOn.Format(time.DateOnly),
		CreatedAt:   in.CreatedAt,
	}
}

func registerIncomes(api huma.API, incomes IncomeService) {
	huma.Register(api, huma.Operation{
		OperationID: "register-income",
		Method:      http.MethodPost,
		Path:        "/periods/{period}/incomes",
		Summary:     "Register an income",
		Description: "Registers money received for the period (salary or extra). An extra income may be " +
			"contributed to a category (category_id), e.g. an exchange gain assigned to a loan.",
		Tags:          []string{tagIncomes},
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusUnprocessableEntity},
	}, func(ctx context.Context, in *RegisterIncomeInput) (*IncomeOutput, error) {
		inc, err := incomes.Register(ctx, income.RegisterInput{
			Period:      in.Period,
			Currency:    string(in.Body.Currency),
			Amount:      in.Body.Amount,
			Kind:        in.Body.Kind,
			Description: in.Body.Description,
			ReceivedOn:  in.Body.ReceivedOn,
			CategoryID:  in.Body.CategoryID,
		})
		if err != nil {
			return nil, toHTTPError(ctx, "register income", err)
		}
		return &IncomeOutput{Body: toIncome(inc)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-incomes",
		Method:      http.MethodGet,
		Path:        "/periods/{period}/incomes",
		Summary:     "List the incomes of a period",
		Description: "Ordered by date, including the ones created by exchanges.",
		Tags:        []string{tagIncomes},
		Errors:      []int{http.StatusUnprocessableEntity},
	}, func(ctx context.Context, in *PeriodParam) (*IncomeListOutput, error) {
		incs, err := incomes.List(ctx, in.Period)
		if err != nil {
			return nil, toHTTPError(ctx, "list incomes", err)
		}
		out := &IncomeListOutput{Body: make([]Income, 0, len(incs))}
		for _, inc := range incs {
			out.Body = append(out.Body, toIncome(inc))
		}
		return out, nil
	})
}

func registerIncomeEdits(api huma.API, incomes IncomeService) {
	huma.Register(api, huma.Operation{
		OperationID: "update-income",
		Method:      http.MethodPatch,
		Path:        "/periods/{period}/incomes/{id}",
		Summary:     "Update an income",
		Description: "Only the sent fields change; the result is validated like a new income. " +
			"Incomes created by exchanges can't be modified (409): delete the exchange instead.",
		Tags:   []string{tagIncomes},
		Errors: []int{http.StatusNotFound, http.StatusConflict, http.StatusUnprocessableEntity},
	}, func(ctx context.Context, in *UpdateIncomeInput) (*IncomeOutput, error) {
		inc, err := incomes.Update(ctx, in.Period, in.ID, income.UpdateInput{
			Currency:    (*string)(in.Body.Currency),
			Amount:      in.Body.Amount,
			Kind:        in.Body.Kind,
			Description: in.Body.Description,
			ReceivedOn:  in.Body.ReceivedOn,
			CategoryID:  in.Body.CategoryID,
		})
		if err != nil {
			return nil, toHTTPError(ctx, "update income", err)
		}
		return &IncomeOutput{Body: toIncome(inc)}, nil
	})

	huma.Register(api, deleteOperation("delete-income", "/periods/{period}/incomes/{id}", "Delete an income",
		"Incomes created by exchanges can't be deleted (409): delete the exchange instead.", tagIncomes),
		func(ctx context.Context, in *EntryParams) (*struct{}, error) {
			if err := incomes.Delete(ctx, in.Period, in.ID); err != nil {
				return nil, toHTTPError(ctx, "delete income", err)
			}
			return nil, nil
		})
}
