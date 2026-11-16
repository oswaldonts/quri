// Command desktop runs Quri as a desktop app: the same application as cmd/api inside a Wails
// window. The API is answered in-process by the asset server handler, so no port is opened.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"github.com/oswaldoaquinas/go-income-mgr/internal/app"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "quri:", err)
		os.Exit(1)
	}
}

func run() error {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx := context.Background()

	configDir, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("locate config directory: %w", err)
	}
	dbPath := filepath.Join(configDir, "go-income-mgr", "income.db")
	if err := applyPendingRestore(dbPath); err != nil {
		return err
	}
	db, err := app.OpenDB(ctx, dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	// No login: the app opens no network port, so the only one who can reach it is whoever is
	// logged into this machine, who owns the database file anyway.
	handler, err := app.New(db, nil, logger, app.Options{NoAuth: true})
	if err != nil {
		return err
	}

	backup := &Backup{db: db, dbPath: dbPath}

	return wails.Run(&options.App{
		Title:     "Quri",
		Width:     1280,
		Height:    820,
		MinWidth:  420,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Handler: handler,
		},
		OnStartup:        backup.startup,
		Bind:             []any{backup},
		BackgroundColour: &options.RGBA{R: 19, G: 19, B: 24, A: 255},
	})
}
