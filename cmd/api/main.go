// Command api starts the income manager HTTP API.
package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/oswaldoaquinas/go-income-mgr/internal/adapter/httpapi"
	"github.com/oswaldoaquinas/go-income-mgr/internal/adapter/memory"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/income"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// Wiring: adapters are created here and injected into the core.
	incomeRepo := memory.NewIncomeRepository()
	incomeService := income.NewService(incomeRepo)

	addr := ":" + getenv("PORT", "8080")

	srv := &http.Server{
		Addr:              addr,
		Handler:           httpapi.NewRouter(incomeService),
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Info("starting server", "addr", addr)
	if err := srv.ListenAndServe(); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

// getenv returns the value of the environment variable key, or fallback if it is unset or empty.
func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
