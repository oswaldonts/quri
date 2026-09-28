// Package auth checks the operator's password and issues session tokens.
//
// There is exactly one operator (this is a single-user app), so there is no
// user store: the password hash and signing secret are configuration, not data.
// Tokens are self-contained (an expiry plus an HMAC over it), so verifying one
// needs no database lookup or server-side session store.
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// SessionTTL is how long an issued token stays valid.
const SessionTTL = 30 * 24 * time.Hour

// ErrWrongPassword means the given password does not match the configured hash.
var ErrWrongPassword = errors.New("wrong password")

// Service checks the operator's password and issues/verifies session tokens.
type Service struct {
	passwordHash []byte
	secret       []byte
}

// NewService builds a Service. passwordHash is a bcrypt hash (see the hashpw
// command); secret signs issued tokens and must stay the same across restarts,
// or every existing session is invalidated.
func NewService(passwordHash, secret []byte) *Service {
	return &Service{passwordHash: passwordHash, secret: secret}
}

// CheckPassword compares password against the configured hash.
func (s *Service) CheckPassword(password string) error {
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
