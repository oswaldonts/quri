// Package app wires the adapters into the core and returns the HTTP handler. Both the server
// (cmd/api) and the desktop build (cmd/desktop) use it, so they run exactly the same application.
package app

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/oswaldoaquinas/go-income-mgr/internal/adapter/httpapi"
	"github.com/oswaldoaquinas/go-income-mgr/internal/adapter/localauth"
	"github.com/oswaldoaquinas/go-income-mgr/internal/adapter/sqlite"
	"github.com/oswaldoaquinas/go-income-mgr/internal/adapter/webui"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/account"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/auth"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/balance"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/budget"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/category"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/currency"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/exchange"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/expense"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/income"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/settings"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/summary"
	"github.com/oswaldoaquinas/go-income-mgr/web"
)

// OpenDB creates the database directory if needed and opens (and migrates) the database.
func OpenDB(ctx context.Context, path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}
	return sqlite.Open(ctx, path)
}

// Options tunes what differs between the server and the desktop build.
type Options struct {
	// NoAuth runs without a login: the desktop build, where the machine's user is the operator.
	NoAuth bool
}

// New builds the handler that serves the API and the embedded frontend from the same origin.
func New(db *sql.DB, authService *auth.Service, logger *slog.Logger, opts Options) (http.Handler, error) {
	categoryRepo := sqlite.NewCategoryRepository(db)
	categoryService := category.NewService(categoryRepo, categoryRepo)
	currencyService := currency.NewService(sqlite.NewCurrencyRepository(db))
	settingsService := settings.NewService(sqlite.NewSettingsRepository(db), currencyService)
	incomeRepo := sqlite.NewIncomeRepository(db)
	expenseRepo := sqlite.NewExpenseRepository(db)
	balanceRepo := sqlite.NewBalanceRepository(db)
	incomeService := income.NewService(incomeRepo, categoryService, currencyService)
	balanceService := balance.NewService(balanceRepo, currencyService)
	accountService := account.NewService(sqlite.NewAccountRepository(db), expenseRepo, currencyService)
	expenseService := expense.NewService(expenseRepo, categoryService, accountService, currencyService)
	budgetRepo := sqlite.NewBudgetRepository(db)
	budgetService := budget.NewService(budgetRepo, categoryService, currencyService)
	summaryService := summary.NewService(incomeRepo, expenseRepo, balanceRepo, categoryRepo, budgetRepo, sqlite.NewPeriodBounds(db))
	exchangeService := exchange.NewService(sqlite.NewRateRepository(db), sqlite.NewExchangeRepository(db), categoryService, accountService, currencyService)

	api := httpapi.NewRouter(httpapi.Services{
		Auth:       authService,
		Categories: categoryService,
		Incomes:    incomeService,
		Expenses:   expenseService,
		Balances:   balanceService,
		Summaries:  summaryService,
		Budgets:    budgetService,
		Rates:      exchangeService,
		Exchanges:  exchangeService,
		Accounts:   accountService,
		Currencies: currencyService,
		Settings:   settingsService,

		NoAuth: opts.NoAuth,
	}, logger)

	// The frontend is embedded in the binary and served from the same origin as the API.
	files := web.Files()
	if _, err := fs.Stat(files, "index.html"); err != nil {
		logger.Warn("web UI not built, serving the API only (build it with: make web-build)")
	}

	return webui.Handler(files, api), nil
}

// LocalAuth loads or, on first run, creates the local credentials
// file next to the rest of this app's data on the current machine.
func LocalAuth() (*auth.Service, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("locate config directory: %w", err)
	}
	store := localauth.NewStore(filepath.Join(configDir, "go-income-mgr", "credentials.json"))

	secret, passwordHash, ok, err := store.Load()
	if err != nil {
		return nil, err
	}
	if !ok {
		if secret, err = store.Init(); err != nil {
			return nil, err
		}
	}
	if passwordHash != nil {
		return auth.NewService(passwordHash, secret), nil
	}
	return auth.NewUnconfiguredService(secret, store.SavePasswordHash(secret)), nil
}
