package account

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/money"
)

const maxDescriptionLength = 200

// TransferInput carries the raw values of a new transfer.
type TransferInput struct {
	From        string // ID of the account the money leaves
	To          string // ID of the account that receives it
	Amount      string // decimal string in the currency of both accounts
	Description string // optional, up to 200 characters
	On          string // YYYY-MM-DD
}

// Transfer moves money between two accounts of the same currency. It is neither an expense nor an
// income: only the balances of the two accounts change.
func (s *Service) Transfer(ctx context.Context, in TransferInput) (Transfer, error) {
	from, err := s.repo.Get(ctx, in.From)
	if errors.Is(err, ErrNotFound) {
		return Transfer{}, invalid(fmt.Errorf("source account: %w", err))
	}
	if err != nil {
		return Transfer{}, fmt.Errorf("get source account: %w", err)
	}
	to, err := s.repo.Get(ctx, in.To)
	if errors.Is(err, ErrNotFound) {
		return Transfer{}, invalid(fmt.Errorf("target account: %w", err))
	}
	if err != nil {
		return Transfer{}, fmt.Errorf("get target account: %w", err)
	}
	if from.ID == to.ID {
		return Transfer{}, invalid(errors.New("source and target are the same account"))
	}
	c := from.Amount.Currency()
	if to.Amount.Currency() != c {
		return Transfer{}, invalid(fmt.Errorf("%s is in %s and %s in %s: use an exchange", from.Name, c, to.Name, to.Amount.Currency()))
	}

	amount, err := money.Parse(in.Amount, c)
	if err != nil {
		return Transfer{}, invalid(err)
	}
	if !amount.IsPositive() {
		return Transfer{}, invalid(errors.New("amount must be greater than zero"))
	}
	description := strings.TrimSpace(in.Description)
	if utf8.RuneCountInString(description) > maxDescriptionLength {
		return Transfer{}, invalid(fmt.Errorf("description longer than %d characters", maxDescriptionLength))
	}
	on, err := time.Parse(time.DateOnly, in.On)
	if err != nil {
		return Transfer{}, invalid(fmt.Errorf("on %q (want YYYY-MM-DD)", in.On))
	}

	t := Transfer{ID: s.newID(), From: from.ID, To: to.ID, Amount: amount, Description: description, On: on, CreatedAt: s.now().UTC()}
	if err := s.repo.SaveTransfer(ctx, t); err != nil {
		return Transfer{}, fmt.Errorf("save transfer: %w", err)
	}
	return t, nil
}

// Transfers returns every transfer, the latest first.
func (s *Service) Transfers(ctx context.Context) ([]Transfer, error) {
	return s.repo.ListTransfers(ctx)
}

// DeleteTransfer removes a transfer; the balances of its accounts go back as if it never happened.
func (s *Service) DeleteTransfer(ctx context.Context, id string) error {
	return s.repo.DeleteTransfer(ctx, id)
}
