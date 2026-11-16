package main

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/oswaldoaquinas/go-income-mgr/internal/app"
)

func newDB(t *testing.T, path string) {
	t.Helper()
	db, err := app.OpenDB(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestStageAcceptsAQuriDatabase(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "backup.db")
	newDB(t, src)

	if err := stage(context.Background(), src, filepath.Join(dir, "income.db.restore")); err != nil {
		t.Fatalf("stage: %v", err)
	}
}

func TestStageRejectsOtherFiles(t *testing.T) {
	dir := t.TempDir()
	notSQLite := filepath.Join(dir, "a.db")
	if err := os.WriteFile(notSQLite, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(dir, "b.db")
	raw, err := sql.Open("sqlite", "file:"+foreign)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec("CREATE TABLE t (x)"); err != nil {
		t.Fatal(err)
	}
	_ = raw.Close()

	for _, src := range []string{notSQLite, foreign} {
		if err := stage(context.Background(), src, filepath.Join(dir, "x.restore")); err == nil {
			t.Errorf("stage(%s) accepted a file that is not a Quri backup", filepath.Base(src))
		}
	}
}

func TestApplyPendingRestoreKeepsTheOldDatabase(t *testing.T) {
	dir := t.TempDir()
	db := filepath.Join(dir, "income.db")
	for name, content := range map[string]string{"income.db": "old", "income.db-wal": "oldwal", "income.db.restore": "new"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if err := applyPendingRestore(db); err != nil {
		t.Fatal(err)
	}

	want := map[string]string{"income.db": "new", "income.db.before-restore": "old", "income.db.before-restore-wal": "oldwal"}
	for name, content := range want {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || string(got) != content {
			t.Errorf("%s = %q, %v; want %q", name, got, err, content)
		}
	}
	for _, name := range []string{"income.db.restore", "income.db-wal"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			t.Errorf("%s still exists", name)
		}
	}
}

func TestApplyPendingRestoreWithoutStagedFileDoesNothing(t *testing.T) {
	dir := t.TempDir()
	db := filepath.Join(dir, "income.db")
	if err := os.WriteFile(db, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := applyPendingRestore(db); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(db); string(got) != "data" {
		t.Errorf("database changed: %q", got)
	}
}
