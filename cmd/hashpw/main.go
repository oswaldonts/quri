// Command hashpw prints the bcrypt hash of a password read from stdin,
// for the AUTH_PASSWORD_HASH environment variable the API expects.
//
// Usage: printf '%s' 'the password' | go run ./cmd/hashpw
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "hashpw:", err)
		os.Exit(1)
	}
}

func run() error {
	password, err := io.ReadAll(bufio.NewReader(os.Stdin))
	if err != nil {
		return fmt.Errorf("read password: %w", err)
	}
	hash, err := bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	fmt.Println(string(hash))
	return nil
}
