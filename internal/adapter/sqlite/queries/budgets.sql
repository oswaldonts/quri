-- name: UpsertBudget :exec
INSERT INTO budgets (period, category_id, currency, amount_minor, updated_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT (period, category_id, currency) DO UPDATE
SET amount_minor = excluded.amount_minor,
    updated_at   = excluded.updated_at;

-- name: ListBudgetsByPeriod :many
SELECT period, category_id, currency, amount_minor, updated_at
FROM budgets
WHERE period = ?
ORDER BY category_id, currency;

-- name: DeleteBudget :execrows
DELETE FROM budgets WHERE period = ? AND category_id = ? AND currency = ?;
