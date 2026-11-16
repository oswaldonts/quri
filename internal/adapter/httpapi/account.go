package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/account"
)

// AccountService is what the HTTP layer needs from the account use cases.
type AccountService interface {
	Create(ctx context.Context, in account.CreateInput) (account.Account, error)
	Update(ctx context.Context, id string, in account.UpdateInput) (account.Account, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context) ([]account.Account, error)
	Transfer(ctx context.Context, in account.TransferInput) (account.Transfer, error)
	Transfers(ctx context.Context) ([]account.Transfer, error)
	DeleteTransfer(ctx context.Context, id string) error
}

// Account is a cash or bank account as returned by the API.
type Account struct {
	ID        string    `json:"id"`
	Name      string    `json:"name" example:"BCP"`
	Kind      string    `json:"kind" enum:"bank,cash"`
	Currency  string    `json:"currency" example:"PEN"`
	Balance   string    `json:"balance" example:"180.00" doc:"What the account holds now: the amount typed, minus the expenses charged to it, plus the transfers and exchanges that moved money into it (minus those that took it out), all registered after it."`
	Amount    string    `json:"amount" example:"200.00" doc:"The amount typed by hand at updated_at."`
	UpdatedAt time.Time `json:"updated_at"`
}

func toAccount(a account.Account) Account {
	return Account{
		ID:        a.ID,
		Name:      a.Name,
		Kind:      string(a.Kind),
		Currency:  string(a.Amount.Currency()),
		Balance:   a.Balance.String(),
		Amount:    a.Amount.String(),
		UpdatedAt: a.UpdatedAt,
	}
}

// AccountOutput wraps an Account as the response body.
type AccountOutput struct {
	Body Account
}

// AccountListOutput is the response of GET /accounts.
type AccountListOutput struct {
	Body []Account
}

// AccountParam is the {id} path parameter of an account.
type AccountParam struct {
	ID string `path:"id"`
}

// CreateAccountInput is the request of POST /accounts.
type CreateAccountInput struct {
	Body struct {
		Name     string       `json:"name,omitempty" maxLength:"40" example:"BCP" doc:"Required for banks; cash is named Efectivo."`
		Kind     string       `json:"kind" enum:"bank,cash"`
		Currency CurrencyCode `json:"currency"`
		Balance  string       `json:"balance" pattern:"^-?[0-9]{1,13}(\\.[0-9]{1,2})?$" example:"200.00" doc:"What the account holds now. May be negative, e.g. a card debt."`
	}
}

// UpdateAccountInput is the request of PATCH /accounts/{id}.
type UpdateAccountInput struct {
	AccountParam
	Body struct {
		Name    *string `json:"name,omitempty" maxLength:"40"`
		Balance *string `json:"balance,omitempty" pattern:"^-?[0-9]{1,13}(\\.[0-9]{1,2})?$" doc:"What the account holds now. For cash this restarts the count: the cash expenses registered so far no longer lower it."`
	}
}

// Transfer is a movement of money between two accounts of the same currency.
type Transfer struct {
	ID          string    `json:"id"`
	From        string    `json:"from_account_id" doc:"Account the money leaves."`
	To          string    `json:"to_account_id" doc:"Account that receives it."`
	Amount      Money     `json:"amount"`
	Description string    `json:"description"`
	On          string    `json:"transferred_on" format:"date" example:"2026-08-01"`
	CreatedAt   time.Time `json:"created_at"`
}

func toTransfer(t account.Transfer) Transfer {
	return Transfer{
		ID:          t.ID,
		From:        t.From,
		To:          t.To,
		Amount:      toMoney(t.Amount),
		Description: t.Description,
		On:          t.On.Format(time.DateOnly),
		CreatedAt:   t.CreatedAt,
	}
}

// TransferOutput wraps a Transfer as the response body.
type TransferOutput struct {
	Body Transfer
}

// TransferListOutput is the response of GET /transfers.
type TransferListOutput struct {
	Body []Transfer
}

// TransferParam is the {id} path parameter of a transfer.
type TransferParam struct {
	ID string `path:"id"`
}

// CreateTransferInput is the request of POST /transfers.
type CreateTransferInput struct {
	Body struct {
		From        string `json:"from_account_id" doc:"Account the money leaves."`
		To          string `json:"to_account_id" doc:"Account that receives it; same currency as the source."`
		Amount      string `json:"amount" pattern:"^[0-9]{1,13}(\\.[0-9]{1,2})?$" example:"500.00"`
		Description string `json:"description,omitempty" maxLength:"200"`
		On          string `json:"transferred_on" format:"date" example:"2026-08-01"`
	}
}

func registerAccounts(api huma.API, accounts AccountService) {
	huma.Register(api, huma.Operation{
		OperationID:   "create-transfer",
		Method:        http.MethodPost,
		Path:          "/transfers",
		Summary:       "Move money between two accounts",
		Description:   "Lowers the balance of the source account and raises the one of the target. It is not an expense nor an income, so no summary counts it. Both accounts must use the same currency; use an exchange otherwise.",
		Tags:          []string{tagAccounts},
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusUnprocessableEntity},
	}, func(ctx context.Context, in *CreateTransferInput) (*TransferOutput, error) {
		t, err := accounts.Transfer(ctx, account.TransferInput{
			From:        in.Body.From,
			To:          in.Body.To,
			Amount:      in.Body.Amount,
			Description: in.Body.Description,
			On:          in.Body.On,
		})
		if err != nil {
			return nil, toHTTPError(ctx, "create transfer", err)
		}
		return &TransferOutput{Body: toTransfer(t)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-transfers",
		Method:      http.MethodGet,
		Path:        "/transfers",
		Summary:     "List the transfers between accounts",
		Description: "The latest first.",
		Tags:        []string{tagAccounts},
	}, func(ctx context.Context, _ *struct{}) (*TransferListOutput, error) {
		ts, err := accounts.Transfers(ctx)
		if err != nil {
			return nil, toHTTPError(ctx, "list transfers", err)
		}
		out := &TransferListOutput{Body: make([]Transfer, 0, len(ts))}
		for _, t := range ts {
			out.Body = append(out.Body, toTransfer(t))
		}
		return out, nil
	})

	huma.Register(api, deleteOperation("delete-transfer", "/transfers/{id}", "Delete a transfer",
		"The balances of its two accounts go back as if it never happened.", tagAccounts),
		func(ctx context.Context, in *TransferParam) (*struct{}, error) {
			if err := accounts.DeleteTransfer(ctx, in.ID); err != nil {
				return nil, toHTTPError(ctx, "delete transfer", err)
			}
			return nil, nil
		})

	huma.Register(api, huma.Operation{
		OperationID:   "create-account",
		Method:        http.MethodPost,
		Path:          "/accounts",
		Summary:       "Add the cash or a bank account",
		Description:   "Informative only: it doesn't change the available money of any period. There is one cash per currency.",
		Tags:          []string{tagAccounts},
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusConflict, http.StatusUnprocessableEntity},
	}, func(ctx context.Context, in *CreateAccountInput) (*AccountOutput, error) {
		a, err := accounts.Create(ctx, account.CreateInput{
			Name:     in.Body.Name,
			Kind:     in.Body.Kind,
			Currency: string(in.Body.Currency),
			Balance:  in.Body.Balance,
		})
		if err != nil {
			return nil, toHTTPError(ctx, "create account", err)
		}
		return &AccountOutput{Body: toAccount(a)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-accounts",
		Method:      http.MethodGet,
		Path:        "/accounts",
		Summary:     "List the cash and the bank accounts",
		Description: "Cash first, then the banks by name, per currency. The balance of each account already discounts the expenses charged to it.",
		Tags:        []string{tagAccounts},
	}, func(ctx context.Context, _ *struct{}) (*AccountListOutput, error) {
		as, err := accounts.List(ctx)
		if err != nil {
			return nil, toHTTPError(ctx, "list accounts", err)
		}
		out := &AccountListOutput{Body: make([]Account, 0, len(as))}
		for _, a := range as {
			out.Body = append(out.Body, toAccount(a))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-account",
		Method:      http.MethodPatch,
		Path:        "/accounts/{id}",
		Summary:     "Rename an account or set what it holds",
		Tags:        []string{tagAccounts},
		Errors:      []int{http.StatusNotFound, http.StatusConflict, http.StatusUnprocessableEntity},
	}, func(ctx context.Context, in *UpdateAccountInput) (*AccountOutput, error) {
		a, err := accounts.Update(ctx, in.ID, account.UpdateInput{Name: in.Body.Name, Balance: in.Body.Balance})
		if err != nil {
			return nil, toHTTPError(ctx, "update account", err)
		}
		return &AccountOutput{Body: toAccount(a)}, nil
	})

	huma.Register(api, deleteOperation("delete-account", "/accounts/{id}", "Delete an account", "", tagAccounts),
		func(ctx context.Context, in *AccountParam) (*struct{}, error) {
			if err := accounts.Delete(ctx, in.ID); err != nil {
				return nil, toHTTPError(ctx, "delete account", err)
			}
			return nil, nil
		})
}
