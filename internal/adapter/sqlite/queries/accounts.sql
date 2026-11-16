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
WHERE account_id = ? AND status = 'confirmed' AND disabled = 0;

-- name: AccountHasExpenses :one
SELECT EXISTS (SELECT 1 FROM expenses WHERE account_id = ?);

-- name: AccountHasTransfers :one
SELECT EXISTS (SELECT 1 FROM transfers WHERE from_account_id = ? OR to_account_id = ?);

-- name: AccountHasExchanges :one
SELECT EXISTS (SELECT 1 FROM exchanges WHERE from_account_id = ? OR to_account_id = ?);

-- name: ListConfirmedExchangeCreditsByAccount :many
SELECT to_amount_minor AS amount_minor, created_at
FROM exchanges
WHERE to_account_id = ? AND status = 'confirmed';

-- name: ListTransfersInByAccount :many
SELECT amount_minor, created_at FROM transfers WHERE to_account_id = ?;

-- name: ListTransfersOutByAccount :many
SELECT amount_minor, created_at FROM transfers WHERE from_account_id = ?;

-- name: InsertTransfer :exec
INSERT INTO transfers (id, from_account_id, to_account_id, currency, amount_minor, description, transferred_on, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListTransfers :many
SELECT id, from_account_id, to_account_id, currency, amount_minor, description, transferred_on, created_at
FROM transfers
ORDER BY transferred_on DESC, created_at DESC;

-- name: DeleteTransfer :execrows
DELETE FROM transfers WHERE id = ?;
