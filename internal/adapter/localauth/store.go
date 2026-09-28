// Package localauth persists the operator's session secret and password hash
// to a local JSON file.
//
// It exists for environments with no deploy pipeline to hand the process
// AUTH_PASSWORD_HASH/AUTH_SESSION_SECRET up front — e.g. a binary someone
// downloaded and runs on their own machine, or the future Wails-packaged
// desktop build. Nomad deployments keep using explicit env vars instead; this
// is only for AUTH_LOCAL_CREDENTIALS=1 (see cmd/api/main.go).
package localauth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// sessionSecretSize is 32 bytes (256 bits): plenty for an HMAC-SHA256 key,
// generated once per Store and reused across restarts.
const sessionSecretSize = 32

// fileFormat is the on-disk JSON shape. PasswordHash is empty until Configure succeeds.
type fileFormat struct {
	SessionSecret string `json:"session_secret"`
	PasswordHash  string `json:"password_hash,omitempty"`
}

// Store reads and writes credentials.json at a fixed path.
type Store struct {
	path string
}

// NewStore builds a Store backed by the file at path. The parent directory is
// created on first write if it doesn't exist.
func NewStore(path string) *Store {
	return &Store{path: path}
}

// Load reads the stored secret and, if Configure already ran, the password
// hash. ok is false when the file doesn't exist yet (first run): call Init.
func (s *Store) Load() (secret, passwordHash []byte, ok bool, err error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil, false, nil
	}
	if err != nil {
		return nil, nil, false, fmt.Errorf("read %s: %w", s.path, err)
	}

	var f fileFormat
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, nil, false, fmt.Errorf("parse %s: %w", s.path, err)
	}
	secret, err = base64.StdEncoding.DecodeString(f.SessionSecret)
	if err != nil {
		return nil, nil, false, fmt.Errorf("decode session secret in %s: %w", s.path, err)
	}
	if f.PasswordHash != "" {
		passwordHash = []byte(f.PasswordHash)
	}
	return secret, passwordHash, true, nil
}

// Init generates a fresh random session secret and writes it to disk with no
// password hash yet. Call it once, when Load reports ok=false.
func (s *Store) Init() (secret []byte, err error) {
	secret = make([]byte, sessionSecretSize)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("generate session secret: %w", err)
	}
	if err := s.write(fileFormat{SessionSecret: base64.StdEncoding.EncodeToString(secret)}); err != nil {
		return nil, err
	}
	return secret, nil
}

// SavePasswordHash records the password hash alongside the existing secret.
// Meant to be passed as the onConfigure callback of auth.NewUnconfiguredService.
func (s *Store) SavePasswordHash(secret []byte) func(passwordHash []byte) error {
	return func(passwordHash []byte) error {
		return s.write(fileFormat{
			SessionSecret: base64.StdEncoding.EncodeToString(secret),
			PasswordHash:  string(passwordHash),
		})
	}
}

// write replaces the file atomically (write to a temp file, then rename) so a
// crash mid-write never leaves a truncated, unparseable credentials file.
func (s *Store) write(f fileFormat) error {
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(s.path), err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("rename %s to %s: %w", tmp, s.path, err)
	}
	return nil
}
