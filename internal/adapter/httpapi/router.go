// Package httpapi exposes the application over HTTP using the chi router.
package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// NewRouter builds the HTTP handler with all routes and middlewares.
func NewRouter(incomes IncomeRegistrar) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", health)

	r.Route("/periods/{period}", func(r chi.Router) {
		r.Post("/incomes", registerIncome(incomes))
	})

	return r
}
