// Package httpapi exposes the application over HTTP using the chi router.
package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Services groups the use cases the HTTP layer depends on.
type Services struct {
	Incomes   IncomeRegistrar
	Rates     RateManager
	Exchanges ExchangeRegistrar
}

// NewRouter builds the HTTP handler with all routes and middlewares.
func NewRouter(s Services) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", health)

	r.Route("/periods/{period}", func(r chi.Router) {
		r.Post("/incomes", registerIncome(s.Incomes))

		r.Put("/rates/{pair}", setRate(s.Rates))
		r.Get("/rates/{pair}", getRate(s.Rates))

		r.Post("/exchanges", registerExchange(s.Exchanges))
	})

	return r
}
