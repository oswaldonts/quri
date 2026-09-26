-- name: InsertExpense :exec
INSERT INTO expenses (id, period, category_id, currency, amount_minor, description, spent_on, exchange_id, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListExpensesByPeriod :many
SELECT id, period, category_id, currency, amount_minor, description, spent_on, exchange_id, created_at
FROM expenses
WHERE period = ?
ORDER BY spent_on, created_at;

-- name: CategoryHasExpenses :one
SELECT EXISTS (
    SELECT 1 FROM expenses WHERE period = ? AND category_id = ?
) AS has_expenses;
