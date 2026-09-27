// Command api starts the income manager HTTP API.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/oswaldoaquinas/go-income-mgr/internal/adapter/httpapi"
	"github.com/oswaldoaquinas/go-income-mgr/internal/adapter/sqlite"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/balance"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/budget"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/category"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/exchange"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/expense"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/income"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/summary"
)

// shutdownTimeout bounds how long in-flight requests may take once a stop signal arrives.
// It must stay below the orchestrator's kill timeout (kill_timeout in the Nomad job).
const shutdownTimeout = 10 * time.Second

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// ctx is canceled on SIGINT (Ctrl+C) or SIGTERM (docker stop, Nomad).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// run returns instead of exiting so its deferred cleanups (closing the database) execute.
	if err := run(ctx, logger); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, logger *slog.Logger) error {
	dbPath := getenv("DATABASE_PATH", "data/income.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o750); err != nil {
		return fmt.Errorf("create database directory: %w", err)
	}
	db, err := sqlite.Open(ctx, dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	// Wiring: adapters are created here and injected into the core.
	categoryRepo := sqlite.NewCategoryRepository(db)
	categoryService := category.NewService(categoryRepo, categoryRepo)
	incomeRepo := sqlite.NewIncomeRepository(db)
	expenseRepo := sqlite.NewExpenseRepository(db)
	balanceRepo := sqlite.NewBalanceRepository(db)
	incomeService := income.NewService(incomeRepo, categoryService)
	balanceService := balance.NewService(balanceRepo)
	expenseService := expense.NewService(expenseRepo, categoryService)
	budgetRepo := sqlite.NewBudgetRepository(db)
	budgetService := budget.NewService(budgetRepo, categoryService)
	summaryService := summary.NewService(incomeRepo, expenseRepo, balanceRepo, categoryRepo, budgetRepo, sqlite.NewPeriodBounds(db))
	exchangeService := exchange.NewService(sqlite.NewRateRepository(db), sqlite.NewExchangeRepository(db), categoryService)

	addr := ":" + getenv("PORT", "8080")
	srv := &http.Server{
		Addr: addr,
		Handler: httpapi.NewRouter(httpapi.Services{
			Categories: categoryService,
			Incomes:    incomeService,
			Expenses:   expenseService,
			Balances:   balanceService,
			Summaries:  summaryService,
			Budgets:    budgetService,
			Rates:      exchangeService,
			Exchanges:  exchangeService,
		}, logger),
		ReadHeaderTimeout: 5 * time.Second,
	}

	serveErr := make(chan error, 1)
	go func() {
		logger.Info("starting server", "addr", addr, "database", dbPath)
		serveErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serveErr: // could not start, e.g. port in use
		return err
	case <-ctx.Done():
	}

	logger.Info("shutting down", "timeout", shutdownTimeout.String())
	// A fresh context: ctx is already canceled, but the shutdown needs its own deadline.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	// Shutdown stops accepting connections and waits for in-flight requests to finish.
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	if err := <-serveErr; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	logger.Info("server stopped")
	return nil
}

// getenv returns the value of the environment variable key, or fallback if it is unset or empty.
func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
