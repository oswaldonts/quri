package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/income"
)

// IncomeRegistrar is what the HTTP layer needs from the income use cases.
// Declaring it here (on the consumer side) lets handler tests use a fake.
type IncomeRegistrar interface {
	Register(ctx context.Context, in income.RegisterInput) (income.Income, error)
}

type registerIncomeRequest struct {
	Currency    string `json:"currency"`
	Amount      string `json:"amount"`
	Kind        string `json:"kind"`
	Description string `json:"description"`
	ReceivedOn  string `json:"received_on"`
}

type incomeResponse struct {
	ID          string    `json:"id"`
	Period      string    `json:"period"`
	SpendMonth  string    `json:"spend_month"`
	Currency    string    `json:"currency"`
	Amount      string    `json:"amount"`
	Kind        string    `json:"kind"`
	Description string    `json:"description"`
	ReceivedOn  string    `json:"received_on"`
	CreatedAt   time.Time `json:"created_at"`
}

func toIncomeResponse(in income.Income) incomeResponse {
	return incomeResponse{
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

// registerIncome handles POST /periods/{period}/incomes.
func registerIncome(incomes IncomeRegistrar) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req registerIncomeRequest
		if err := decodeJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
			return
		}

		inc, err := incomes.Register(r.Context(), income.RegisterInput{
			Period:      chi.URLParam(r, "period"),
			Currency:    req.Currency,
			Amount:      req.Amount,
			Kind:        req.Kind,
			Description: req.Description,
			ReceivedOn:  req.ReceivedOn,
		})
		switch {
		case errors.Is(err, income.ErrInvalidInput):
			writeError(w, http.StatusBadRequest, err.Error())
			return
		case err != nil:
			slog.ErrorContext(r.Context(), "register income", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		writeJSON(w, http.StatusCreated, toIncomeResponse(inc))
	}
}
