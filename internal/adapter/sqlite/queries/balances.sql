-- name: UpsertBalance :exec
INSERT INTO balances (period, currency, amount_minor, updated_at)
VALUES (?, ?, ?, ?)
ON CONFLICT (period, currency) DO UPDATE
SET amount_minor = excluded.amount_minor,
    updated_at   = excluded.updated_at;

-- name: ListBalancesByPeriod :many
SELECT period, currency, amount_minor, updated_at
FROM balances
WHERE period = ?
ORDER BY currency;
