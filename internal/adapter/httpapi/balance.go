package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/balance"
)

// BalanceService is what the HTTP layer needs from the balance use cases.
type BalanceService interface {
	Set(ctx context.Context, in balance.SetInput) (balance.Balance, error)
	List(ctx context.Context, period string) ([]balance.Balance, error)
	Delete(ctx context.Context, period, currency string) error
}

// BalanceParams identify the balance of a currency in a period.
type BalanceParams struct {
	PeriodParam
	Currency CurrencyCode `path:"currency"`
}

// SetBalanceInput is the request of PUT /periods/{period}/balances/{currency}.
type SetBalanceInput struct {
	PeriodParam
	Currency CurrencyCode `path:"currency"`
	Body     struct {
		Amount string `json:"amount" pattern:"^-?[0-9]{1,13}(\\.[0-9]{1,2})?$" example:"415.50" doc:"Real money in the currency (accounts plus cash). May be negative, e.g. a credit card debt."`
	}
}

// Balance is a real balance as returned by the API.
type Balance struct {
	Period    string    `json:"period" example:"2026-07"`
	Currency  string    `json:"currency" example:"PEN"`
	Amount    string    `json:"amount" example:"415.50"`
	UpdatedAt time.Time `json:"updated_at"`
}

func toBalance(b balance.Balance) Balance {
	return Balance{
		Period:    b.Period.String(),
		Currency:  string(b.Amount.Currency()),
		Amount:    b.Amount.String(),
		UpdatedAt: b.UpdatedAt,
	}
}

// BalanceOutput wraps a Balance as the response body.
type BalanceOutput struct {
	Body Balance
}

// BalanceListOutput is the response of GET /periods/{period}/balances.
type BalanceListOutput struct {
	Body []Balance
}

func registerBalances(api huma.API, balances BalanceService) {
	huma.Register(api, deleteOperation("delete-balance", "/periods/{period}/balances/{currency}",
		"Delete the real balance of a currency", "", tagBalances),
		func(ctx context.Context, in *BalanceParams) (*struct{}, error) {
			if err := balances.Delete(ctx, in.Period, string(in.Currency)); err != nil {
				return nil, toHTTPError(ctx, "delete balance", err)
			}
			return nil, nil
		})

	huma.Register(api, huma.Operation{
		OperationID: "set-balance",
		Method:      http.MethodPut,
		Path:        "/periods/{period}/balances/{currency}",
		Summary:     "Set the real balance of a currency",
		Description: "The money that really exists in the currency at the end of the period. " +
			"The summary compares it with the expected balance to detect unregistered entries.",
		Tags:   []string{tagBalances},
		Errors: []int{http.StatusUnprocessableEntity},
	}, func(ctx context.Context, in *SetBalanceInput) (*BalanceOutput, error) {
		b, err := balances.Set(ctx, balance.SetInput{
			Period:   in.Period,
			Currency: string(in.Currency),
			Amount:   in.Body.Amount,
		})
		if err != nil {
			return nil, toHTTPError(ctx, "set balance", err)
		}
		return &BalanceOutput{Body: toBalance(b)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-balances",
		Method:      http.MethodGet,
		Path:        "/periods/{period}/balances",
		Summary:     "List the real balances of a period",
		Tags:        []string{tagBalances},
		Errors:      []int{http.StatusUnprocessableEntity},
	}, func(ctx context.Context, in *PeriodParam) (*BalanceListOutput, error) {
		bs, err := balances.List(ctx, in.Period)
		if err != nil {
			return nil, toHTTPError(ctx, "list balances", err)
		}
		out := &BalanceListOutput{Body: make([]Balance, 0, len(bs))}
		for _, b := range bs {
			out.Body = append(out.Body, toBalance(b))
		}
		return out, nil
	})
}
