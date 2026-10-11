package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/expense"
)

// ExpenseService is what the HTTP layer needs from the expense use cases.
type ExpenseService interface {
	Register(ctx context.Context, in expense.RegisterInput) (expense.Expense, error)
	RegisterBatch(ctx context.Context, in expense.BatchInput) ([]expense.Expense, error)
	List(ctx context.Context, period string, f expense.Filter) ([]expense.Expense, error)
	Update(ctx context.Context, period, id string, in expense.UpdateInput) (expense.Expense, error)
	Delete(ctx context.Context, period, id string) error
}

// UpdateExpenseInput is the request of PATCH /periods/{period}/expenses/{id}: only the sent fields change.
type UpdateExpenseInput struct {
	EntryParams
	Body struct {
		CategoryID  *string       `json:"category_id,omitempty"`
		Currency    *CurrencyCode `json:"currency,omitempty"`
		Amount      *string       `json:"amount,omitempty" pattern:"^[0-9]{1,13}(\\.[0-9]{1,2})?$"`
		Description *string       `json:"description,omitempty" maxLength:"200"`
		Tags        *[]string     `json:"tags,omitempty" maxItems:"10" doc:"Replaces all the tags; an empty list removes them."`
		SpentOn     *string       `json:"spent_on,omitempty" format:"date"`
		Status      *string       `json:"status,omitempty" enum:"planned,confirmed" doc:"Set confirmed when a planned entry happens; amount and date can be corrected in the same request."`
		AccountID   *string       `json:"account_id,omitempty" doc:"Account the money comes out of (see /accounts), in the same currency as the expense. Its balance goes down by the amount once confirmed. Empty removes it."`
	}
}

// RegisterBatchInput is the request of POST /periods/{period}/expenses/batch.
type RegisterBatchInput struct {
	PeriodParam
	Body struct {
		CategoryID string       `json:"category_id,omitempty" example:"market" doc:"Common category, used by items without their own."`
		Currency   CurrencyCode `json:"currency,omitempty" doc:"Common currency, used by items without their own."`
		SpentOn    string       `json:"spent_on,omitempty" format:"date" example:"2026-08-03" doc:"Common date, used by items without their own."`
		Tags       []string     `json:"tags,omitempty" maxItems:"10" doc:"Added to the tags of every item."`
		Status     string       `json:"status,omitempty" enum:"planned,confirmed" doc:"planned: expected, not happened yet (only counts in view=projected). confirmed (default): happened."`
		Items      []BatchItem  `json:"items" minItems:"1" maxItems:"100"`
	}
}

// BatchItem is one line of a batch; empty fields take the common value.
type BatchItem struct {
	Description string       `json:"description,omitempty" maxLength:"200" example:"Chocolate Milky"`
	Amount      string       `json:"amount" pattern:"^[0-9]{1,13}(\\.[0-9]{1,2})?$" example:"27"`
	CategoryID  string       `json:"category_id,omitempty"`
	Currency    CurrencyCode `json:"currency,omitempty"`
	SpentOn     string       `json:"spent_on,omitempty" format:"date"`
	Tags        []string     `json:"tags,omitempty" maxItems:"10" example:"[\"dulces\"]"`
	Status      string       `json:"status,omitempty" enum:"planned,confirmed" doc:"Overrides the common status for this item."`
}

// ListExpensesInput is the request of GET /periods/{period}/expenses.
type ListExpensesInput struct {
	PeriodParam
	CategoryID string `query:"category_id" example:"market" doc:"Only expenses of this category."`
	Tag        string `query:"tag" example:"dulces" doc:"Only expenses with this tag."`
	Status     string `query:"status" enum:"planned,confirmed" doc:"Only planned or only confirmed expenses."`
}

// RegisterExpenseInput is the request of POST /periods/{period}/expenses.
type RegisterExpenseInput struct {
	PeriodParam
	Body struct {
		CategoryID  string       `json:"category_id" example:"market" doc:"Must be used in the period (see GET /periods/{period}/categories)."`
		Currency    CurrencyCode `json:"currency"`
		Amount      string       `json:"amount" pattern:"^[0-9]{1,13}(\\.[0-9]{1,2})?$" example:"45.50" doc:"Decimal string with up to 2 decimal places."`
		Description string       `json:"description,omitempty" maxLength:"200" example:"Chocolate Milky"`
		Tags        []string     `json:"tags,omitempty" maxItems:"10" example:"[\"dulces\"]" doc:"Kinds of expense, e.g. dulces or lácteos. Normalized to lowercase; a leading # is dropped."`
		Status      string       `json:"status,omitempty" enum:"planned,confirmed" doc:"planned: expected, not happened yet (only counts in view=projected). confirmed (default): happened."`
		SpentOn     string       `json:"spent_on" format:"date" example:"2026-08-03"`
		AccountID   string       `json:"account_id,omitempty" doc:"Account the money comes out of (see /accounts), in the same currency as the expense. Its balance goes down by the amount once confirmed."`
	}
}

// Expense is an expense as returned by the API.
type Expense struct {
	ID          string    `json:"id"`
	Period      string    `json:"period" example:"2026-07"`
	CategoryID  string    `json:"category_id" example:"market"`
	Currency    string    `json:"currency" example:"PEN"`
	Amount      string    `json:"amount" example:"45.50"`
	Description string    `json:"description"`
	Tags        []string  `json:"tags"`
	Status      string    `json:"status" enum:"planned,confirmed"`
	SpentOn     string    `json:"spent_on" format:"date" example:"2026-08-03"`
	AccountID   string    `json:"account_id,omitempty" doc:"Account the money came out of (see /accounts); its balance goes down by the amount."`
	Cash        bool      `json:"cash" doc:"True when the account is the cash: the expense is left out of the period summary, whose available money is what is in the banks."`
	ExchangeID  string    `json:"exchange_id,omitempty" doc:"Set when the expense is the source side of an exchange."`
	CreatedAt   time.Time `json:"created_at"`
}

func toExpense(e expense.Expense) Expense {
	return Expense{
		ID:          e.ID,
		Period:      e.Period.String(),
		CategoryID:  e.CategoryID,
		Currency:    string(e.Amount.Currency()),
		Amount:      e.Amount.String(),
		Description: e.Description,
		Tags:        append([]string{}, e.Tags...), // never null in JSON
		Status:      string(e.Status),
		SpentOn:     e.SpentOn.Format(time.DateOnly),
		AccountID:   e.AccountID,
		Cash:        e.Cash,
		ExchangeID:  e.ExchangeID,
		CreatedAt:   e.CreatedAt,
	}
}

// ExpenseOutput wraps an Expense as the response body.
type ExpenseOutput struct {
	Body Expense
}

// ExpenseListOutput is the response of GET /periods/{period}/expenses.
type ExpenseListOutput struct {
	Body []Expense
}

func registerExpenses(api huma.API, expenses ExpenseService) {
	huma.Register(api, huma.Operation{
		OperationID: "register-expense",
		Method:      http.MethodPost,
		Path:        "/periods/{period}/expenses",
		Summary:     "Register an expense",
		Description: "The category must be used in the period. The first expense of a period fixes its category list. " +
			"Expenses in the system category \"exchange\" are created by POST /periods/{period}/exchanges.",
		Tags:          []string{tagExpenses},
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusUnprocessableEntity},
	}, func(ctx context.Context, in *RegisterExpenseInput) (*ExpenseOutput, error) {
		e, err := expenses.Register(ctx, expense.RegisterInput{
			Period:      in.Period,
			CategoryID:  in.Body.CategoryID,
			Currency:    string(in.Body.Currency),
			Amount:      in.Body.Amount,
			Description: in.Body.Description,
			Tags:        in.Body.Tags,
			SpentOn:     in.Body.SpentOn,
			Status:      in.Body.Status,
			AccountID:   in.Body.AccountID,
		})
		if err != nil {
			return nil, toHTTPError(ctx, "register expense", err)
		}
		return &ExpenseOutput{Body: toExpense(e)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-expenses",
		Method:      http.MethodGet,
		Path:        "/periods/{period}/expenses",
		Summary:     "List the expenses of a period",
		Description: "Ordered by date, including the ones created by exchanges. Filters can be combined.",
		Tags:        []string{tagExpenses},
		Errors:      []int{http.StatusUnprocessableEntity},
	}, func(ctx context.Context, in *ListExpensesInput) (*ExpenseListOutput, error) {
		es, err := expenses.List(ctx, in.Period, expense.Filter{CategoryID: in.CategoryID, Tag: in.Tag, Status: in.Status})
		if err != nil {
			return nil, toHTTPError(ctx, "list expenses", err)
		}
		out := &ExpenseListOutput{Body: make([]Expense, 0, len(es))}
		for _, e := range es {
			out.Body = append(out.Body, toExpense(e))
		}
		return out, nil
	})
}

func registerExpenseBatch(api huma.API, expenses ExpenseService) {
	huma.Register(api, huma.Operation{
		OperationID: "register-expense-batch",
		Method:      http.MethodPost,
		Path:        "/periods/{period}/expenses/batch",
		Summary:     "Register several expenses at once",
		Description: "For example the items of a grocery receipt. Common fields apply to items that don't set " +
			"their own; common tags are added to each item's tags. Every item is validated before saving and " +
			"either all are saved or none is; errors name the failing item.",
		Tags:          []string{tagExpenses},
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusUnprocessableEntity},
	}, func(ctx context.Context, in *RegisterBatchInput) (*ExpenseListOutput, error) {
		items := make([]expense.BatchItem, 0, len(in.Body.Items))
		for _, it := range in.Body.Items {
			items = append(items, expense.BatchItem{
				Description: it.Description,
				Amount:      it.Amount,
				CategoryID:  it.CategoryID,
				Currency:    string(it.Currency),
				SpentOn:     it.SpentOn,
				Tags:        it.Tags,
				Status:      it.Status,
			})
		}
		es, err := expenses.RegisterBatch(ctx, expense.BatchInput{
			Period:     in.Period,
			CategoryID: in.Body.CategoryID,
			Currency:   string(in.Body.Currency),
			SpentOn:    in.Body.SpentOn,
			Tags:       in.Body.Tags,
			Status:     in.Body.Status,
			Items:      items,
		})
		if err != nil {
			return nil, toHTTPError(ctx, "register expense batch", err)
		}
		out := &ExpenseListOutput{Body: make([]Expense, 0, len(es))}
		for _, e := range es {
			out.Body = append(out.Body, toExpense(e))
		}
		return out, nil
	})
}

func registerExpenseEdits(api huma.API, expenses ExpenseService) {
	huma.Register(api, huma.Operation{
		OperationID: "update-expense",
		Method:      http.MethodPatch,
		Path:        "/periods/{period}/expenses/{id}",
		Summary:     "Update an expense",
		Description: "Only the sent fields change; the result is validated like a new expense. " +
			"Expenses created by exchanges can't be modified (409): delete the exchange instead.",
		Tags:   []string{tagExpenses},
		Errors: []int{http.StatusNotFound, http.StatusConflict, http.StatusUnprocessableEntity},
	}, func(ctx context.Context, in *UpdateExpenseInput) (*ExpenseOutput, error) {
		e, err := expenses.Update(ctx, in.Period, in.ID, expense.UpdateInput{
			CategoryID:  in.Body.CategoryID,
			Currency:    (*string)(in.Body.Currency),
			Amount:      in.Body.Amount,
			Description: in.Body.Description,
			Tags:        in.Body.Tags,
			SpentOn:     in.Body.SpentOn,
			Status:      in.Body.Status,
			AccountID:   in.Body.AccountID,
		})
		if err != nil {
			return nil, toHTTPError(ctx, "update expense", err)
		}
		return &ExpenseOutput{Body: toExpense(e)}, nil
	})

	huma.Register(api, deleteOperation("delete-expense", "/periods/{period}/expenses/{id}", "Delete an expense",
		"Expenses created by exchanges can't be deleted (409): delete the exchange instead.", tagExpenses),
		func(ctx context.Context, in *EntryParams) (*struct{}, error) {
			if err := expenses.Delete(ctx, in.Period, in.ID); err != nil {
				return nil, toHTTPError(ctx, "delete expense", err)
			}
			return nil, nil
		})
}
