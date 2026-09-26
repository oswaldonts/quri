-- name: InsertIncome :exec
INSERT INTO incomes (id, period, currency, amount_minor, kind, description, received_on, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);
