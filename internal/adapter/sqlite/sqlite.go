// Package sqlite implements the core repositories on SQLite.
//
// Queries live in queries/*.sql and the schema in migrations/*.sql; sqlc turns
// them into the type-safe code of the sqlitedb package (docker compose run --rm
// sqlc generate). Migrations are embedded in the binary and applied on Open.
package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"time"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite" // pure Go driver (no CGO), registered as "sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Open opens the database file (creating it if needed) and applies pending migrations.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	dsn := "file:" + path +
		"?_pragma=foreign_keys(1)" + // SQLite doesn't enforce REFERENCES unless enabled
		"&_pragma=journal_mode(WAL)" + // readers don't block the writer
		"&_pragma=busy_timeout(5000)" // wait up to 5s for a lock instead of failing

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	// SQLite allows a single writer; one connection avoids "database is locked" errors.
	db.SetMaxOpenConns(1)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	if err := migrate(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func migrate(ctx context.Context, db *sql.DB) error {
	fsys, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return fmt.Errorf("migrations: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, db, fsys)
	if err != nil {
		return fmt.Errorf("migrations: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

// Timestamps are stored as RFC 3339 text in UTC.

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func parseTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, s)
}
