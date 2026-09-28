package auth

import (
	"errors"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func newTestService(t *testing.T, password string) *Service {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("GenerateFromPassword: %v", err)
	}
	return NewService(hash, []byte("test-secret"))
}

func TestCheckPassword(t *testing.T) {
	s := newTestService(t, "correct horse")

	if err := s.CheckPassword("correct horse"); err != nil {
		t.Errorf("CheckPassword(correct) = %v, want nil", err)
	}
	if err := s.CheckPassword("wrong"); !errors.Is(err, ErrWrongPassword) {
		t.Errorf("CheckPassword(wrong) = %v, want %v", err, ErrWrongPassword)
	}
}

func TestIssueAndVerifyToken(t *testing.T) {
	s := newTestService(t, "correct horse")

	token, expiresAt := s.IssueToken()
	if !expiresAt.After(time.Now()) {
		t.Fatalf("IssueToken expiry %v is not in the future", expiresAt)
	}
	if !s.VerifyToken(token) {
		t.Error("VerifyToken(issued token) = false, want true")
	}
}

func TestVerifyTokenRejectsTamperedOrForeign(t *testing.T) {
	s := newTestService(t, "correct horse")
	other := &Service{passwordHash: s.passwordHash, secret: []byte("other-secret")}
	token, _ := s.IssueToken()

	cases := map[string]string{
		"empty":           "",
		"garbage":         "not-a-token",
		"missing-dot":     "12345",
		"signed-by-other": func() string { t, _ := other.IssueToken(); return t }(),
		"truncated":       token[:len(token)-1],
	}
	for name, tok := range cases {
		if s.VerifyToken(tok) {
			t.Errorf("VerifyToken(%s) = true, want false", name)
		}
	}
}

func TestVerifyTokenRejectsExpired(t *testing.T) {
	s := newTestService(t, "correct horse")
	payload := "1" // 1970-01-01, long expired
	token := payload + "." + s.sign(payload)

	if s.VerifyToken(token) {
		t.Error("VerifyToken(expired token) = true, want false")
	}
}

func TestUnconfiguredServiceRejectsPasswordUntilConfigured(t *testing.T) {
	s := NewUnconfiguredService([]byte("test-secret"), nil)

	if s.Configured() {
		t.Fatal("Configured() = true before Configure was ever called")
	}
	if err := s.CheckPassword("anything"); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("CheckPassword before Configure = %v, want %v", err, ErrNotConfigured)
	}

	if err := s.Configure("correct horse"); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if !s.Configured() {
		t.Error("Configured() = false after Configure succeeded")
	}
	if err := s.CheckPassword("correct horse"); err != nil {
		t.Errorf("CheckPassword(correct) after Configure = %v, want nil", err)
	}
	if err := s.CheckPassword("wrong"); !errors.Is(err, ErrWrongPassword) {
		t.Errorf("CheckPassword(wrong) after Configure = %v, want %v", err, ErrWrongPassword)
	}
}

func TestConfigureOnlyWorksOnce(t *testing.T) {
	s := NewUnconfiguredService([]byte("test-secret"), nil)
	if err := s.Configure("first"); err != nil {
		t.Fatalf("first Configure: %v", err)
	}
	if err := s.Configure("second"); !errors.Is(err, ErrAlreadyConfigured) {
		t.Errorf("second Configure = %v, want %v", err, ErrAlreadyConfigured)
	}
	// The first password must still be the one in effect.
	if err := s.CheckPassword("first"); err != nil {
		t.Errorf("CheckPassword(first) = %v, want nil", err)
	}
}

func TestConfigurePersistsBeforeTakingEffect(t *testing.T) {
	persisted := false
	failing := NewUnconfiguredService([]byte("test-secret"), func([]byte) error {
		return errors.New("disk full")
	})
	if err := failing.Configure("pw"); err == nil {
		t.Fatal("Configure must fail when onConfigure fails")
	}
	if failing.Configured() {
		t.Error("Configured() = true after a failed persist")
	}

	ok := NewUnconfiguredService([]byte("test-secret"), func([]byte) error {
		persisted = true
		return nil
	})
	if err := ok.Configure("pw"); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if !persisted {
		t.Error("onConfigure was never called")
	}
}

// Already-configured Services (the env-var deployment mode) reject Configure too.
func TestConfiguredServiceRejectsConfigure(t *testing.T) {
	s := newTestService(t, "correct horse")
	if err := s.Configure("anything"); !errors.Is(err, ErrAlreadyConfigured) {
		t.Errorf("Configure on an already-configured Service = %v, want %v", err, ErrAlreadyConfigured)
	}
}
