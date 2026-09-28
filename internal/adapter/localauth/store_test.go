package localauth

import (
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	return NewStore(filepath.Join(t.TempDir(), "nested", "credentials.json"))
}

func TestLoadReportsNotOkBeforeInit(t *testing.T) {
	s := newTestStore(t)

	_, _, ok, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if ok {
		t.Error("Load ok = true before Init was ever called")
	}
}

func TestInitThenLoadRoundTripsTheSecret(t *testing.T) {
	s := newTestStore(t)

	secret, err := s.Init()
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if len(secret) != sessionSecretSize {
		t.Fatalf("Init secret length = %d, want %d", len(secret), sessionSecretSize)
	}

	gotSecret, gotHash, ok, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !ok {
		t.Fatal("Load ok = false right after Init")
	}
	if string(gotSecret) != string(secret) {
		t.Errorf("Load secret = %x, want %x", gotSecret, secret)
	}
	if gotHash != nil {
		t.Errorf("Load password hash = %q, want nil before any SavePasswordHash", gotHash)
	}
}

func TestSavePasswordHashKeepsTheSameSecret(t *testing.T) {
	s := newTestStore(t)
	secret, err := s.Init()
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	save := s.SavePasswordHash(secret)
	if err := save([]byte("$2a$10$fakehash")); err != nil {
		t.Fatalf("SavePasswordHash: %v", err)
	}

	gotSecret, gotHash, ok, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !ok {
		t.Fatal("Load ok = false after SavePasswordHash")
	}
	if string(gotSecret) != string(secret) {
		t.Errorf("Load secret changed after SavePasswordHash: got %x, want %x", gotSecret, secret)
	}
	if string(gotHash) != "$2a$10$fakehash" {
		t.Errorf("Load password hash = %q, want %q", gotHash, "$2a$10$fakehash")
	}
}
