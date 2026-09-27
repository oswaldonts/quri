// Package httpapi exposes the application over HTTP.
//
// Routes are registered with Huma on top of the chi router: each operation is
// described with Go structs and tags, from which Huma validates requests and
// generates the OpenAPI 3.1 document (/openapi.yaml) and the docs (/docs).
package httpapi

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

const (
	apiTitle   = "go-income-mgr API"
	apiVersion = "0.4.0"
)

const apiDescription = `Personal income and expense manager.

Budgets are organized by **period**: the month the money was earned (YYYY-MM).
That money is spent during the following month (spend_month).

**Amounts** are decimal strings ("1330.00"), never JSON numbers, to avoid floating
point rounding. Currencies are ISO 4217 codes.

**Exchanges** convert money with the rate set for the period:
target = source × rate, truncated to the target currency's minor unit. The converted
amount is registered automatically as an income of kind "exchange".`

// Services groups the use cases the HTTP layer depends on.
type Services struct {
	Categories CategoryCatalog
	Incomes    IncomeService
	Expenses   ExpenseService
	Balances   BalanceService
	Summaries  SummaryService
	Budgets    BudgetService
	Rates      RateManager
	Exchanges  ExchangeService
}

// NewRouter builds the HTTP handler with all routes, middlewares and docs.
func NewRouter(s Services) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	api := humachi.New(r, newConfig())

	registerHealth(api)
	registerCategories(api, s.Categories)
	registerIncomes(api, s.Incomes)
	registerIncomeEdits(api, s.Incomes)
	registerExpenses(api, s.Expenses)
	registerExpenseBatch(api, s.Expenses)
	registerExpenseEdits(api, s.Expenses)
	registerBalances(api, s.Balances)
	registerSummary(api, s.Summaries)
	registerAvailable(api, s.Summaries)
	registerTrends(api, s.Summaries)
	registerBudgets(api, s.Budgets)
	registerRates(api, s.Rates)
	registerExchanges(api, s.Exchanges)

	return r
}

func newConfig() huma.Config {
	config := huma.DefaultConfig(apiTitle, apiVersion)
	config.Info.Description = apiDescription
	config.DocsRenderer = huma.DocsRendererSwaggerUI
	config.Servers = []*huma.Server{{URL: "/", Description: "Current server"}}

	// The default hooks add a "$schema" field to every response body; keep responses clean.
	config.CreateHooks = nil

	config.Tags = []*huma.Tag{
		{Name: tagSystem, Description: "Service status."},
		{Name: tagCategories, Description: "Global catalog of expense categories. Categories are archived, never deleted."},
		{Name: tagIncomes, Description: "Money received in a period."},
		{Name: tagExpenses, Description: "Money spent in a period, by category."},
		{Name: tagBalances, Description: "Closing check: the money you really have per currency (accounts plus cash), " +
			"entered by hand at the end of the month. Not to be confused with available, which the API computes " +
			"(income − spent). The summary compares both: difference = available − real should be 0; otherwise " +
			"an income or expense is missing or wrong."},
		{Name: tagSummary, Description: "Overview of a period computed from its entries."},
		{Name: tagBudgets, Description: "Planned amount per category, compared with what was spent."},
		{Name: tagRates, Description: "Exchange rate chosen for each period and currency pair."},
		{Name: tagExchanges, Description: "Currency conversions made with the period rate."},
	}
	return config
}

const (
	tagSystem     = "System"
	tagCategories = "Categories"
	tagIncomes    = "Incomes"
	tagExpenses   = "Expenses"
	tagBalances   = "Balances"
	tagSummary    = "Summary"
	tagBudgets    = "Budgets"
	tagRates      = "Rates"
	tagExchanges  = "Exchanges"
)
