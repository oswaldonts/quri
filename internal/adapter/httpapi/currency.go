package httpapi

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/currency"
)

// CurrencyService is what the HTTP layer needs from the currency use cases.
type CurrencyService interface {
	Create(ctx context.Context, in currency.CreateInput) (currency.Currency, error)
	Update(ctx context.Context, code string, symbol string) (currency.Currency, error)
	Delete(ctx context.Context, code string) error
	List(ctx context.Context) ([]currency.Currency, error)
}

// Currency is a currency registered by the user.
type Currency struct {
	Code   string `json:"code" example:"EUR"`
	Symbol string `json:"symbol" example:"€" doc:"Shown before amounts. Empty: the code is shown."`
}

func toCurrency(c currency.Currency) Currency {
	return Currency{Code: string(c.Code), Symbol: c.Symbol}
}

// CurrencyOutput wraps a Currency as the response body.
type CurrencyOutput struct {
	Body Currency
}

// CurrencyListOutput is the response of GET /currencies.
type CurrencyListOutput struct {
	Body []Currency
}

// CurrencyParam is the {code} path parameter of a currency.
type CurrencyParam struct {
	Code string `path:"code" pattern:"^[A-Z]{3}$" example:"EUR"`
}

// CreateCurrencyInput is the request of POST /currencies.
type CreateCurrencyInput struct {
	Body struct {
		Code   string `json:"code" pattern:"^[A-Za-z]{3}$" example:"EUR" doc:"Three-letter code, e.g. an ISO 4217 one."`
		Symbol string `json:"symbol,omitempty" maxLength:"16" example:"€"`
	}
}

// UpdateCurrencyInput is the request of PATCH /currencies/{code}.
type UpdateCurrencyInput struct {
	CurrencyParam
	Body struct {
		Symbol string `json:"symbol" maxLength:"16" example:"€"`
	}
}

func registerCurrencies(api huma.API, currencies CurrencyService) {
	huma.Register(api, huma.Operation{
		OperationID:   "create-currency",
		Method:        http.MethodPost,
		Path:          "/currencies",
		Summary:       "Register a currency",
		Description:   "Amounts can only be in registered currencies. A few common ones come registered; add the ones you use.",
		Tags:          []string{tagCurrencies},
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusConflict, http.StatusUnprocessableEntity},
	}, func(ctx context.Context, in *CreateCurrencyInput) (*CurrencyOutput, error) {
		c, err := currencies.Create(ctx, currency.CreateInput{Code: in.Body.Code, Symbol: in.Body.Symbol})
		if err != nil {
			return nil, toHTTPError(ctx, "create currency", err)
		}
		return &CurrencyOutput{Body: toCurrency(c)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-currencies",
		Method:      http.MethodGet,
		Path:        "/currencies",
		Summary:     "List the registered currencies",
		Tags:        []string{tagCurrencies},
	}, func(ctx context.Context, _ *struct{}) (*CurrencyListOutput, error) {
		cs, err := currencies.List(ctx)
		if err != nil {
			return nil, toHTTPError(ctx, "list currencies", err)
		}
		out := &CurrencyListOutput{Body: make([]Currency, 0, len(cs))}
		for _, c := range cs {
			out.Body = append(out.Body, toCurrency(c))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-currency",
		Method:      http.MethodPatch,
		Path:        "/currencies/{code}",
		Summary:     "Change the symbol of a currency",
		Tags:        []string{tagCurrencies},
		Errors:      []int{http.StatusNotFound, http.StatusUnprocessableEntity},
	}, func(ctx context.Context, in *UpdateCurrencyInput) (*CurrencyOutput, error) {
		c, err := currencies.Update(ctx, in.Code, in.Body.Symbol)
		if err != nil {
			return nil, toHTTPError(ctx, "update currency", err)
		}
		return &CurrencyOutput{Body: toCurrency(c)}, nil
	})

	huma.Register(api, deleteOperation("delete-currency", "/currencies/{code}", "Remove a currency",
		"Only while no income, expense, account, balance, budget, rate or exchange uses it.", tagCurrencies),
		func(ctx context.Context, in *CurrencyParam) (*struct{}, error) {
			if err := currencies.Delete(ctx, in.Code); err != nil {
				return nil, toHTTPError(ctx, "delete currency", err)
			}
			return nil, nil
		})
}
