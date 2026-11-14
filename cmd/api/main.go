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
	"syscall"
	"time"

	"github.com/oswaldoaquinas/go-income-mgr/internal/app"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/auth"
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
	authService, err := newAuthService()
	if err != nil {
		return err
	}

	dbPath := getenv("DATABASE_PATH", "data/income.db")
	db, err := app.OpenDB(ctx, dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	handler, err := app.New(db, authService, logger)
	if err != nil {
		return err
	}

	addr := ":" + getenv("PORT", "8080")
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
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

// newAuthService picks one of two credential sources, and fails fast if
// neither is set up: there is no insecure default for a service that guards
// financial data.
//
//   - AUTH_PASSWORD_HASH + AUTH_SESSION_SECRET: the deployed mode (Nomad passes
//     these explicitly; see deploy/nomad/go-income-mgr.nomad.hcl). Both required
//     together so a half-set pair fails loudly instead of silently degrading.
//   - AUTH_LOCAL_CREDENTIALS=1: for a binary with no deploy pipeline behind it
//     (e.g. downloaded and run on someone's own machine, or the Wails desktop
//     build). Credentials live in a local file; POST /setup sets the password
//     the first time it runs.
func newAuthService() (*auth.Service, error) {
	hash := os.Getenv("AUTH_PASSWORD_HASH")
	secret := os.Getenv("AUTH_SESSION_SECRET")
	switch {
	case hash != "" && secret != "":
		return auth.NewService([]byte(hash), []byte(secret)), nil
	case hash != "" || secret != "":
		return nil, errors.New("set both AUTH_PASSWORD_HASH and AUTH_SESSION_SECRET, or neither " +
			"(with AUTH_LOCAL_CREDENTIALS=1) to use the local setup flow")
	case getenv("AUTH_LOCAL_CREDENTIALS", "") == "1":
		return app.LocalAuth()
	default:
		return nil, errors.New("set AUTH_PASSWORD_HASH and AUTH_SESSION_SECRET (deployed mode), or " +
			"AUTH_LOCAL_CREDENTIALS=1 to store credentials in a local file instead " +
			"(generate a password hash with: printf '%s' 'password' | go run ./cmd/hashpw)")
	}
}
