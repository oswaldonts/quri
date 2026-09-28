// Package auth checks the operator's password and issues session tokens.
//
// There is exactly one operator (this is a single-user app), so there is no
// user store: the password hash and signing secret are configuration, not
// data. Tokens are self-contained (an expiry plus an HMAC over it), so
// verifying one needs no database lookup or server-side session store.
//
// A Service can start unconfigured (no password yet) for environments with no
// deploy pipeline to hand it AUTH_PASSWORD_HASH up front — e.g. a binary
// someone downloaded and runs on their own machine for the first time. See
// NewUnconfiguredService.
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// SessionTTL is how long an issued token stays valid.
const SessionTTL = 30 * 24 * time.Hour

var (
	// ErrWrongPassword means the given password does not match the configured hash.
	ErrWrongPassword = errors.New("wrong password")
	// ErrNotConfigured means Configure has not been called yet: there is no password to check.
	ErrNotConfigured = errors.New("no password set yet")
	// ErrAlreadyConfigured means Configure was already called once; it never overwrites a password.
	ErrAlreadyConfigured = errors.New("password already set")
)

// Service checks the operator's password and issues/verifies session tokens.
// The zero value is not usable; construct one with NewService or NewUnconfiguredService.
type Service struct {
	secret []byte // fixed at construction, needs no lock to read

	mu           sync.RWMutex
	passwordHash []byte // nil until Configure succeeds
	onConfigure  func(passwordHash []byte) error
}

// NewService builds an already-configured Service. passwordHash is a bcrypt
// hash (see the hashpw command); secret signs issued tokens and must stay the
// same across restarts, or every existing session is invalidated.
func NewService(passwordHash, secret []byte) *Service {
	return &Service{passwordHash: passwordHash, secret: secret}
}

// NewUnconfiguredService builds a Service with no password yet: CheckPassword
// fails with ErrNotConfigured until Configure is called. onConfigure, if not
// nil, is called with the freshly hashed password when Configure succeeds, so
// the caller can persist it (e.g. to a local file); Configure fails if it does.
func NewUnconfiguredService(secret []byte, onConfigure func(passwordHash []byte) error) *Service {
	return &Service{secret: secret, onConfigure: onConfigure}
}

// Configured reports whether a password has been set.
func (s *Service) Configured() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.passwordHash != nil
}

// Configure sets the operator password. It only works once: it returns
// ErrAlreadyConfigured if a password is already set, and never overwrites one
// (changing the password means editing the stored hash directly).
func (s *Service) Configure(password string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.passwordHash != nil {
		return ErrAlreadyConfigured
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if s.onConfigure != nil {
		if err := s.onConfigure(hash); err != nil {
			return err
		}
	}
	s.passwordHash = hash
	return nil
}

// CheckPassword compares password against the configured hash.
func (s *Service) CheckPassword(password string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.passwordHash == nil {
		return ErrNotConfigured
	}
	if err := bcrypt.CompareHashAndPassword(s.passwordHash, []byte(password)); err != nil {
		return ErrWrongPassword
	}
	return nil
}

// IssueToken returns a new session token, valid for SessionTTL, and its expiry.
func (s *Service) IssueToken() (token string, expiresAt time.Time) {
	expiresAt = time.Now().Add(SessionTTL)
	payload := strconv.FormatInt(expiresAt.Unix(), 10)
	return payload + "." + s.sign(payload), expiresAt
}

// VerifyToken reports whether token was issued by IssueToken and has not expired.
func (s *Service) VerifyToken(token string) bool {
	payload, sig, ok := strings.Cut(token, ".")
	if !ok || !hmac.Equal([]byte(sig), []byte(s.sign(payload))) {
		return false
	}
	expiresAtUnix, err := strconv.ParseInt(payload, 10, 64)
	if err != nil {
		return false
	}
	return time.Now().Before(time.Unix(expiresAtUnix, 0))
}

func (s *Service) sign(payload string) string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
