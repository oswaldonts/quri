package httpapi

import (
	"context"
	"errors"
	"log/slog"

	"github.com/danielgtaylor/huma/v2"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/category"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/exchange"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/expense"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/income"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/money"
)

// PeriodParam is the {period} path parameter shared by every period route.
type PeriodParam struct {
	Period string `path:"period" pattern:"^[0-9]{4}-(0[1-9]|1[0-2])$" example:"2026-07" doc:"Month the money was earned (YYYY-MM)."`
}

// Money is an amount in a currency.
type Money struct {
	Currency string `json:"currency" example:"USD" doc:"ISO 4217 currency code."`
	Amount   string `json:"amount" example:"1330.00" doc:"Decimal string."`
}

func toMoney(m money.Money) Money {
	return Money{Currency: string(m.Currency()), Amount: m.String()}
}

// toHTTPError converts core errors into RFC 9457 problem responses.
// Unexpected errors are logged and hidden from the client.
func toHTTPError(ctx context.Context, op string, err error) error {
	switch {
	case errors.Is(err, income.ErrInvalidInput),
		errors.Is(err, exchange.ErrInvalidInput),
		errors.Is(err, category.ErrInvalidInput),
		errors.Is(err, expense.ErrInvalidInput):
		return huma.Error422UnprocessableEntity(err.Error())
	case errors.Is(err, category.ErrNotFound):
		return huma.Error404NotFound(err.Error())
	case errors.Is(err, exchange.ErrRateNotFound),
		errors.Is(err, category.ErrNameTaken),
		errors.Is(err, category.ErrSystemCategory),
		errors.Is(err, category.ErrArchived),
		errors.Is(err, category.ErrInUse):
		// The request is valid but conflicts with the current state.
		return huma.Error409Conflict(err.Error())
	default:
		slog.ErrorContext(ctx, op, "error", err)
		return huma.Error500InternalServerError("internal error")
	}
}
