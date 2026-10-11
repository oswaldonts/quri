package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/account"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/auth"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/balance"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/budget"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/category"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/exchange"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/expense"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/income"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/money"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/summary"
)

// EntryParams identify one entry ({id}) of a period.
type EntryParams struct {
	PeriodParam
	ID string `path:"id"`
}

// deleteOperation describes a DELETE that answers 204 No Content.
func deleteOperation(id, path, summary, description, tag string) huma.Operation {
	return huma.Operation{
		OperationID:   id,
		Method:        http.MethodDelete,
		Path:          path,
		Summary:       summary,
		Description:   description,
		Tags:          []string{tag},
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{http.StatusNotFound, http.StatusConflict, http.StatusUnprocessableEntity},
	}
}

// PeriodParam is the {period} path parameter shared by every period route.
type PeriodParam struct {
	Period string `path:"period" pattern:"^[0-9]{4}-(0[1-9]|1[0-2])$" example:"2026-07" doc:"Month the money was earned (YYYY-MM)."`
}

// CurrencyCode is an ISO 4217 code in request bodies. Its schema enum is built from
// money.Currencies(), so supporting a new currency never requires touching the API.
type CurrencyCode string

// Schema implements huma.SchemaProvider.
func (CurrencyCode) Schema(huma.Registry) *huma.Schema {
	codes := money.Currencies()
	enum := make([]any, 0, len(codes))
	for _, c := range codes {
		enum = append(enum, string(c))
	}
	return &huma.Schema{
		Type:        huma.TypeString,
		Enum:        enum,
		Description: "ISO 4217 currency code.",
		Examples:    []any{string(money.USD)},
	}
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
	case errors.Is(err, auth.ErrWrongPassword):
		return huma.Error401Unauthorized(err.Error())
	case errors.Is(err, auth.ErrNotConfigured):
		return huma.Error428PreconditionRequired(err.Error())
	case errors.Is(err, auth.ErrAlreadyConfigured):
		return huma.Error409Conflict(err.Error())
	case errors.Is(err, income.ErrInvalidInput),
		errors.Is(err, exchange.ErrInvalidInput),
		errors.Is(err, category.ErrInvalidInput),
		errors.Is(err, expense.ErrInvalidInput),
		errors.Is(err, account.ErrInvalidInput),
		errors.Is(err, balance.ErrInvalidInput),
		errors.Is(err, summary.ErrInvalidInput),
		errors.Is(err, budget.ErrInvalidInput):
		return huma.Error422UnprocessableEntity(err.Error())
	case errors.Is(err, category.ErrNotFound),
		errors.Is(err, income.ErrNotFound),
		errors.Is(err, expense.ErrNotFound),
		errors.Is(err, exchange.ErrNotFound),
		errors.Is(err, balance.ErrNotFound),
		errors.Is(err, account.ErrNotFound),
		errors.Is(err, budget.ErrNotFound):
		return huma.Error404NotFound(err.Error())
	case errors.Is(err, exchange.ErrRateNotFound),
		errors.Is(err, category.ErrNameTaken),
		errors.Is(err, account.ErrNameTaken),
		errors.Is(err, account.ErrCashExists),
		errors.Is(err, account.ErrInUse),
		errors.Is(err, category.ErrSystemCategory),
		errors.Is(err, category.ErrArchived),
		errors.Is(err, category.ErrInUse),
		errors.Is(err, income.ErrManagedByExchange),
		errors.Is(err, expense.ErrManagedByExchange):
		// The request is valid but conflicts with the current state.
		return huma.Error409Conflict(err.Error())
	default:
		slog.ErrorContext(ctx, op, "error", err, "request_id", middleware.GetReqID(ctx))
		return huma.Error500InternalServerError("internal error")
	}
}
