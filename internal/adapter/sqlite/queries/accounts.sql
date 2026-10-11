-- name: InsertAccount :exec
INSERT INTO accounts (id, name, kind, currency, amount_minor, updated_at)
VALUES (?, ?, ?, ?, ?, ?);

-- name: GetAccount :one
SELECT id, name, kind, currency, amount_minor, updated_at
FROM accounts
WHERE id = ?;

-- Cash first, then banks by name.
-- name: ListAccounts :many
SELECT id, name, kind, currency, amount_minor, updated_at
FROM accounts
ORDER BY currency, kind DESC, name;

-- name: UpdateAccount :execrows
UPDATE accounts
SET name = ?, amount_minor = ?, updated_at = ?
WHERE id = ?;

-- name: DeleteAccount :execrows
DELETE FROM accounts WHERE id = ?;

-- Timestamps are text of variable length, so the caller compares them as times.
-- name: ListConfirmedExpensesByAccount :many
SELECT amount_minor, created_at
FROM expenses
WHERE account_id = ? AND status = 'confirmed';

-- name: AccountHasExpenses :one
SELECT EXISTS (SELECT 1 FROM expenses WHERE account_id = ?);
