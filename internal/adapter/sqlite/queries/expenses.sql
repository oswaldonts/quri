-- name: InsertExpense :exec
INSERT INTO expenses (id, period, category_id, currency, amount_minor, description, spent_on, exchange_id, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListExpensesByPeriod :many
SELECT id, period, category_id, currency, amount_minor, description, spent_on, exchange_id, created_at
FROM expenses
WHERE period = ?
ORDER BY spent_on, created_at;

-- Expenses or contributions (extra incomes assigned to the category) in the period.
-- name: CategoryHasEntries :one
SELECT EXISTS (SELECT 1 FROM expenses WHERE expenses.period = sqlc.arg(period) AND expenses.category_id = sqlc.arg(category_id))
    OR EXISTS (SELECT 1 FROM incomes WHERE incomes.period = sqlc.arg(period) AND incomes.category_id = sqlc.arg(category_id))
    AS has_entries;

-- name: InsertExpenseTag :exec
INSERT INTO expense_tags (expense_id, tag) VALUES (?, ?);

-- name: ListExpenseTagsByPeriod :many
SELECT expense_tags.expense_id, expense_tags.tag
FROM expense_tags
JOIN expenses ON expenses.id = expense_tags.expense_id
WHERE expenses.period = ?
ORDER BY expense_tags.expense_id, expense_tags.tag;
