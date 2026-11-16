package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/oswaldoaquinas/go-income-mgr/internal/app"
)

const (
	restoreSuffix = ".restore"
	beforeSuffix  = ".before-restore"
	confirmLabel  = "Restaurar"
	cancelLabel   = "Cancelar"
)

var dbFilter = runtime.FileFilter{DisplayName: "Copia de Quri (*.db)", Pattern: "*.db"}

// Backup is bound to the frontend (window.go.main.Backup): the file dialogs are native, which a
// webview cannot offer reliably through plain downloads and uploads.
type Backup struct {
	ctx    context.Context
	db     *sql.DB
	dbPath string
}

func (b *Backup) startup(ctx context.Context) { b.ctx = ctx }

// ExportBackup asks where to save a consistent snapshot of the database. It returns the chosen
// path, or "" when the user cancelled.
func (b *Backup) ExportBackup() (string, error) {
	path, err := runtime.SaveFileDialog(b.ctx, runtime.SaveDialogOptions{
		Title:           "Guardar copia de seguridad",
		DefaultFilename: "quri-" + time.Now().Format("2006-01-02") + ".db",
		Filters:         []runtime.FileFilter{dbFilter},
	})
	if err != nil || path == "" {
		return "", err
	}
	// VACUUM INTO refuses to overwrite; the dialog already asked the user about replacing it.
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("no se pudo reemplazar el archivo: %w", err)
	}
	if _, err := b.db.ExecContext(b.ctx, "VACUUM INTO ?", path); err != nil {
		return "", fmt.Errorf("no se pudo crear la copia: %w", err)
	}
	return path, nil
}

// ImportBackup asks for a backup file, checks it and stages it; the swap happens the next time
// the app starts (applyPendingRestore), because the open database cannot be replaced under the
// running app. It returns true when the app is quitting to finish the restore.
func (b *Backup) ImportBackup() (bool, error) {
	src, err := runtime.OpenFileDialog(b.ctx, runtime.OpenDialogOptions{
		Title:   "Elegir copia de seguridad",
		Filters: []runtime.FileFilter{dbFilter},
	})
	if err != nil || src == "" {
		return false, err
	}

	answer, err := runtime.MessageDialog(b.ctx, runtime.MessageDialogOptions{
		Type:          runtime.QuestionDialog,
		Title:         "Restaurar copia",
		Message:       "Los datos actuales serán reemplazados por los de la copia. Los actuales se guardan junto a la base de datos (income.db.before-restore). Quri se cerrará al terminar.",
		Buttons:       []string{confirmLabel, cancelLabel},
		DefaultButton: cancelLabel,
		CancelButton:  cancelLabel,
	})
	if err != nil || answer != confirmLabel {
		return false, err
	}

	staged := b.dbPath + restoreSuffix
	if err := stage(b.ctx, src, staged); err != nil {
		_ = os.Remove(staged)
		return false, fmt.Errorf("el archivo no es una copia válida de Quri: %w", err)
	}
	_, _ = runtime.MessageDialog(b.ctx, runtime.MessageDialogOptions{
		Type:    runtime.InfoDialog,
		Title:   "Copia lista",
		Message: "Quri se cerrará ahora. Ábrela de nuevo para ver los datos restaurados.",
	})
	runtime.Quit(b.ctx)
	return true, nil
}

// stage copies src to dst and checks it is an intact Quri database, bringing it to the schema of
// this version (migrations run on open).
func stage(ctx context.Context, src, dst string) error {
	if err := copyFile(src, dst); err != nil {
		return err
	}
	raw, err := sql.Open("sqlite", "file:"+dst+"?mode=ro")
	if err != nil {
		return err
	}
	var check string
	err = raw.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&check)
	if err == nil && check != "ok" {
		err = errors.New(check)
	}
	if err == nil {
		var n int
		err = raw.QueryRowContext(ctx, "SELECT count(*) FROM goose_db_version").Scan(&n)
		if err == nil {
			err = raw.QueryRowContext(ctx, "SELECT count(*) FROM incomes").Scan(&n)
		}
	}
	_ = raw.Close()
	if err != nil {
		return err
	}

	db, err := app.OpenDB(ctx, dst)
	if err != nil {
		return err
	}
	return db.Close()
}

// applyPendingRestore swaps in a staged backup before the database is opened. The replaced
// database (and its WAL files) is kept as income.db.before-restore*.
func applyPendingRestore(dbPath string) error {
	staged := dbPath + restoreSuffix
	if _, err := os.Stat(staged); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	for _, ext := range []string{"", "-wal", "-shm"} {
		old := dbPath + ext
		if err := os.Rename(old, dbPath+beforeSuffix+ext); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("keep the current database: %w", err)
		}
	}
	if err := os.Rename(staged, dbPath); err != nil {
		return fmt.Errorf("restore the backup: %w", err)
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
