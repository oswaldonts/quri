package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/exchange"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/money"
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

type setRateRequest struct {
	Rate string `json:"rate"`
}

type rateResponse struct {
	Period    string    `json:"period"`
	From      string    `json:"from"`
	To        string    `json:"to"`
	Rate      string    `json:"rate"`
	UpdatedAt time.Time `json:"updated_at"`
}

func toRateResponse(r exchange.PeriodRate) rateResponse {
	return rateResponse{
		Period:    r.Period.String(),
		From:      string(r.From),
		To:        string(r.To),
		Rate:      r.Rate.String(),
		UpdatedAt: r.UpdatedAt,
	}
}

type registerExchangeRequest struct {
	From        string `json:"from"`
	To          string `json:"to"`
	Amount      string `json:"amount"`
	Description string `json:"description"`
	ExchangedOn string `json:"exchanged_on"`
}

type moneyResponse struct {
	Currency string `json:"currency"`
	Amount   string `json:"amount"`
}

func toMoneyResponse(m money.Money) moneyResponse {
	return moneyResponse{Currency: string(m.Currency()), Amount: m.String()}
}

type exchangeResponse struct {
	ID          string        `json:"id"`
	Period      string        `json:"period"`
	From        moneyResponse `json:"from"`
	To          moneyResponse `json:"to"`
	Rate        string        `json:"rate"`
	Description string        `json:"description"`
	ExchangedOn string        `json:"exchanged_on"`
	IncomeID    string        `json:"income_id"`
	CreatedAt   time.Time     `json:"created_at"`
}

func toExchangeResponse(e exchange.Exchange) exchangeResponse {
	return exchangeResponse{
		ID:          e.ID,
		Period:      e.Period.String(),
		From:        toMoneyResponse(e.From),
		To:          toMoneyResponse(e.To),
		Rate:        e.Rate.String(),
		Description: e.Description,
		ExchangedOn: e.ExchangedOn.Format(time.DateOnly),
		IncomeID:    e.IncomeID,
		CreatedAt:   e.CreatedAt,
	}
}

// pairInput reads {period} and {pair} (e.g. "USD-PEN") from the URL.
func pairInput(r *http.Request) (exchange.PairInput, bool) {
	from, to, ok := strings.Cut(chi.URLParam(r, "pair"), "-")
	return exchange.PairInput{Period: chi.URLParam(r, "period"), From: from, To: to}, ok
}

// setRate handles PUT /periods/{period}/rates/{pair}.
func setRate(rates RateManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pair, ok := pairInput(r)
		if !ok {
			writeError(w, http.StatusBadRequest, "pair must look like USD-PEN")
			return
		}
		var req setRateRequest
		if err := decodeJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
			return
		}

		pr, err := rates.SetRate(r.Context(), exchange.SetRateInput{PairInput: pair, Rate: req.Rate})
		if err != nil {
			writeExchangeError(w, r, err, "set rate")
			return
		}
		writeJSON(w, http.StatusOK, toRateResponse(pr))
	}
}

// getRate handles GET /periods/{period}/rates/{pair}.
func getRate(rates RateManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pair, ok := pairInput(r)
		if !ok {
			writeError(w, http.StatusBadRequest, "pair must look like USD-PEN")
			return
		}

		pr, err := rates.Rate(r.Context(), pair)
		if errors.Is(err, exchange.ErrRateNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		if err != nil {
			writeExchangeError(w, r, err, "get rate")
			return
		}
		writeJSON(w, http.StatusOK, toRateResponse(pr))
	}
}

// registerExchange handles POST /periods/{period}/exchanges.
func registerExchange(exchanges ExchangeRegistrar) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req registerExchangeRequest
		if err := decodeJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
			return
		}

		ex, err := exchanges.Register(r.Context(), exchange.RegisterInput{
			PairInput:   exchange.PairInput{Period: chi.URLParam(r, "period"), From: req.From, To: req.To},
			Amount:      req.Amount,
			Description: req.Description,
			ExchangedOn: req.ExchangedOn,
		})
		if err != nil {
			writeExchangeError(w, r, err, "register exchange")
			return
		}
		writeJSON(w, http.StatusCreated, toExchangeResponse(ex))
	}
}

// writeExchangeError maps exchange errors to HTTP status codes.
func writeExchangeError(w http.ResponseWriter, r *http.Request, err error, op string) {
	switch {
	case errors.Is(err, exchange.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, exchange.ErrRateNotFound):
		// The request is valid but can't be processed until the rate is set.
		writeError(w, http.StatusConflict, err.Error())
	default:
		slog.ErrorContext(r.Context(), op, "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}
