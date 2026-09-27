-- name: InsertIncome :exec
INSERT INTO incomes (id, period, currency, amount_minor, kind, description, received_on, created_at, category_id)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListIncomesByPeriod :many
SELECT id, period, currency, amount_minor, kind, description, received_on, created_at, category_id
FROM incomes
WHERE period = ?
ORDER BY received_on, created_at;

-- name: GetIncome :one
SELECT id, period, currency, amount_minor, kind, description, received_on, created_at, category_id
FROM incomes
WHERE id = ?;

-- name: UpdateIncome :execrows
UPDATE incomes
SET currency = ?, amount_minor = ?, kind = ?, description = ?, received_on = ?, category_id = ?
WHERE id = ?;

-- name: DeleteIncome :execrows
DELETE FROM incomes WHERE id = ?;
