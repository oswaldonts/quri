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

	authService, err := app.LocalAuth()
	if err != nil {
		return err
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("locate config directory: %w", err)
	}
	db, err := app.OpenDB(ctx, filepath.Join(configDir, "go-income-mgr", "income.db"))
	if err != nil {
		return err
	}
	defer db.Close()

	// Secure cookies need https or localhost; the webview serves from wails:// (or wails.localhost),
	// and the app has no network port for anyone to sniff.
	handler, err := app.New(db, authService, logger, app.Options{InsecureCookies: true})
	if err != nil {
		return err
	}

	return wails.Run(&options.App{
		Title:     "Quri",
		Width:     1280,
		Height:    820,
		MinWidth:  420,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Handler: handler,
		},
		BackgroundColour: &options.RGBA{R: 19, G: 19, B: 24, A: 255},
	})
}
