-- name: UpsertRate :exec
INSERT INTO rates (period, from_currency, to_currency, rate_scaled, updated_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT (period, from_currency, to_currency) DO UPDATE
SET rate_scaled = excluded.rate_scaled,
    updated_at  = excluded.updated_at;

-- name: GetRate :one
SELECT period, from_currency, to_currency, rate_scaled, updated_at
FROM rates
WHERE period = ? AND from_currency = ? AND to_currency = ?;

-- name: InsertExchange :exec
INSERT INTO exchanges (
    id, period, from_currency, from_amount_minor, to_currency, to_amount_minor,
    rate_scaled, description, exchanged_on, income_id, created_at, status, from_account_id, to_account_id
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetExchange :one
SELECT id, period, from_currency, from_amount_minor, to_currency, to_amount_minor,
       rate_scaled, description, exchanged_on, income_id, created_at, status, from_account_id, to_account_id
FROM exchanges
WHERE id = ?;

-- name: ListExchangesByPeriod :many
SELECT id, period, from_currency, from_amount_minor, to_currency, to_amount_minor,
       rate_scaled, description, exchanged_on, income_id, created_at, status, from_account_id, to_account_id
FROM exchanges
WHERE period = ?
ORDER BY exchanged_on, created_at;

-- name: DeleteExchange :exec
DELETE FROM exchanges WHERE id = ?;

-- name: DeleteRate :execrows
DELETE FROM rates WHERE period = ? AND from_currency = ? AND to_currency = ?;

-- name: UpdateExchange :execrows
UPDATE exchanges
SET from_amount_minor = ?, to_amount_minor = ?, description = ?, exchanged_on = ?, status = ?,
    from_account_id = ?, to_account_id = ?
WHERE id = ?;
