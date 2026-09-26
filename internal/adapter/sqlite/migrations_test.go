package sqlite

import (
	"context"
	"database/sql"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/pressly/goose/v3"
)

// TestBackfillExchangeExpenses checks that migration 00004 creates the "exchange"
// expense of exchanges registered before it existed.
func TestBackfillExchangeExpenses(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "test.db")+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	fsys, _ := fs.Sub(migrations, "migrations")
	provider, err := goose.NewProvider(goose.DialectSQLite3, db, fsys)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(ctx, 3); err != nil {
		t.Fatalf("migrate to 3: %v", err)
	}

	// An exchange as it was stored before expenses existed.
	for _, stmt := range []string{
		`INSERT INTO incomes VALUES ('inc-1', '2026-07', 'PEN', 356200, 'exchange', '', '2026-08-01', '2026-08-01T10:00:00Z')`,
		`INSERT INTO exchanges VALUES ('ex-1', '2026-07', 'USD', 109600, 'PEN', 356200, 32500, 'Casa de cambio', '2026-08-01', 'inc-1', '2026-08-01T10:00:00Z')`,
	} {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("migrate to latest: %v", err)
	}

	var category, currency, exchangeID string
	var amount int64
	err = db.QueryRowContext(ctx, `SELECT category_id, currency, amount_minor, exchange_id FROM expenses`).
		Scan(&category, &currency, &amount, &exchangeID)
	if err != nil {
		t.Fatalf("read backfilled expense: %v", err)
	}
	if category != "exchange" || currency != "USD" || amount != 109600 || exchangeID != "ex-1" {
		t.Errorf("backfilled expense = %s %s %d %s, want exchange USD 109600 ex-1", category, currency, amount, exchangeID)
	}
}
