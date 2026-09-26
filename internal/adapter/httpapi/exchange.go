package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/exchange"
)

// RateManager is what the HTTP layer needs to manage period rates.
type RateManager interface {
	SetRate(ctx context.Context, in exchange.SetRateInput) (exchange.PeriodRate, error)
	Rate(ctx context.Context, in exchange.PairInput) (exchange.PeriodRate, error)
}

// ExchangeRegistrar is what the HTTP layer needs to register exchanges.
type ExchangeRegistrar interface {
	Register(ctx context.Context, in exchange.RegisterInput) (exchange.Exchange, error)
}

// PairParams are the {period} and {pair} path parameters of the rate routes.
type PairParams struct {
	PeriodParam
	Pair string `path:"pair" pattern:"^[A-Z]{3}-[A-Z]{3}$" example:"USD-PEN" doc:"Source and target currency: 1 source = rate × target."`
}

func (p PairParams) toInput() exchange.PairInput {
	from, to, _ := strings.Cut(p.Pair, "-") // the pattern guarantees the dash
	return exchange.PairInput{Period: p.Period, From: from, To: to}
}

// SetRateInput is the request of PUT /periods/{period}/rates/{pair}.
type SetRateInput struct {
	PairParams
	Body struct {
		Rate string `json:"rate" pattern:"^[0-9]{1,13}(\\.[0-9]{1,4})?$" example:"3.25" doc:"Positive decimal with up to 4 decimal places."`
	}
}

// GetRateInput is the request of GET /periods/{period}/rates/{pair}.
type GetRateInput struct {
	PairParams
}

// Rate is a period rate as returned by the API.
type Rate struct {
	Period    string    `json:"period" example:"2026-07"`
	From      string    `json:"from" example:"USD"`
	To        string    `json:"to" example:"PEN"`
	Rate      string    `json:"rate" example:"3.2500" doc:"Always formatted with 4 decimal places."`
	UpdatedAt time.Time `json:"updated_at"`
}

// RateOutput wraps a Rate as the response body.
type RateOutput struct {
	Body Rate
}

func toRate(r exchange.PeriodRate) Rate {
	return Rate{
		Period:    r.Period.String(),
		From:      string(r.From),
		To:        string(r.To),
		Rate:      r.Rate.String(),
		UpdatedAt: r.UpdatedAt,
	}
}

// RegisterExchangeInput is the request of POST /periods/{period}/exchanges.
type RegisterExchangeInput struct {
	PeriodParam
	Body struct {
		From        string `json:"from" enum:"USD,PEN" doc:"Source currency (spent)."`
		To          string `json:"to" enum:"USD,PEN" doc:"Target currency (received)."`
		Amount      string `json:"amount" pattern:"^[0-9]{1,13}(\\.[0-9]{1,2})?$" example:"1096.00" doc:"Amount spent in the source currency."`
		Description string `json:"description,omitempty" maxLength:"200" example:"Casa de cambio"`
		ExchangedOn string `json:"exchanged_on" format:"date" example:"2026-08-01"`
	}
}

// Exchange is an exchange as returned by the API.
type Exchange struct {
	ID          string    `json:"id"`
	Period      string    `json:"period" example:"2026-07"`
	From        Money     `json:"from"`
	To          Money     `json:"to"`
	Rate        string    `json:"rate" example:"3.2500"`
	Description string    `json:"description"`
	ExchangedOn string    `json:"exchanged_on" format:"date" example:"2026-08-01"`
	IncomeID    string    `json:"income_id" doc:"ID of the exchange income created in the target currency."`
	CreatedAt   time.Time `json:"created_at"`
}

// ExchangeOutput wraps an Exchange as the response body.
type ExchangeOutput struct {
	Body Exchange
}

func toExchange(e exchange.Exchange) Exchange {
	return Exchange{
		ID:          e.ID,
		Period:      e.Period.String(),
		From:        toMoney(e.From),
		To:          toMoney(e.To),
		Rate:        e.Rate.String(),
		Description: e.Description,
		ExchangedOn: e.ExchangedOn.Format(time.DateOnly),
		IncomeID:    e.IncomeID,
		CreatedAt:   e.CreatedAt,
	}
}

func registerRates(api huma.API, rates RateManager) {
	huma.Register(api, huma.Operation{
		OperationID: "set-rate",
		Method:      http.MethodPut,
		Path:        "/periods/{period}/rates/{pair}",
		Summary:     "Set the rate of a period",
		Description: "Creates or replaces the rate. Exchanges already registered keep the rate they were made with.",
		Tags:        []string{tagRates},
		Errors:      []int{http.StatusUnprocessableEntity},
	}, func(ctx context.Context, in *SetRateInput) (*RateOutput, error) {
		pr, err := rates.SetRate(ctx, exchange.SetRateInput{PairInput: in.toInput(), Rate: in.Body.Rate})
		if err != nil {
			return nil, toHTTPError(ctx, "set rate", err)
		}
		return &RateOutput{Body: toRate(pr)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-rate",
		Method:      http.MethodGet,
		Path:        "/periods/{period}/rates/{pair}",
		Summary:     "Get the rate of a period",
		Tags:        []string{tagRates},
		Errors:      []int{http.StatusNotFound, http.StatusUnprocessableEntity},
	}, func(ctx context.Context, in *GetRateInput) (*RateOutput, error) {
		pr, err := rates.Rate(ctx, in.toInput())
		if errors.Is(err, exchange.ErrRateNotFound) {
			return nil, huma.Error404NotFound(err.Error())
		}
		if err != nil {
			return nil, toHTTPError(ctx, "get rate", err)
		}
		return &RateOutput{Body: toRate(pr)}, nil
	})
}

func registerExchanges(api huma.API, exchanges ExchangeRegistrar) {
	huma.Register(api, huma.Operation{
		OperationID: "register-exchange",
		Method:      http.MethodPost,
		Path:        "/periods/{period}/exchanges",
		Summary:     "Register an exchange",
		Description: "Converts amount with the period rate (truncating), stores the exchange and registers " +
			"the converted amount as an income of kind \"exchange\" in the target currency. " +
			"Returns 409 if the period has no rate for the pair.",
		Tags:          []string{tagExchanges},
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusConflict, http.StatusUnprocessableEntity},
	}, func(ctx context.Context, in *RegisterExchangeInput) (*ExchangeOutput, error) {
		ex, err := exchanges.Register(ctx, exchange.RegisterInput{
			PairInput:   exchange.PairInput{Period: in.Period, From: in.Body.From, To: in.Body.To},
			Amount:      in.Body.Amount,
			Description: in.Body.Description,
			ExchangedOn: in.Body.ExchangedOn,
		})
		if err != nil {
			return nil, toHTTPError(ctx, "register exchange", err)
		}
		return &ExchangeOutput{Body: toExchange(ex)}, nil
	})
}
