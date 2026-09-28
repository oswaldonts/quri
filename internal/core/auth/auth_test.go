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
