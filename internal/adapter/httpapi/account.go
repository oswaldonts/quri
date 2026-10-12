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
}

// Account is a cash or bank account as returned by the API.
type Account struct {
	ID        string    `json:"id"`
	Name      string    `json:"name" example:"BCP"`
	Kind      string    `json:"kind" enum:"bank,cash"`
	Currency  string    `json:"currency" example:"PEN"`
	Balance   string    `json:"balance" example:"180.00" doc:"What the account holds now. For cash: the amount typed minus the cash expenses registered after it."`
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

func registerAccounts(api huma.API, accounts AccountService) {
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
