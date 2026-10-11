-- name: InsertCurrency :exec
INSERT INTO currencies (code, symbol) VALUES (?, ?);

-- name: GetCurrency :one
SELECT code, symbol FROM currencies WHERE code = ?;

-- name: ListCurrencies :many
SELECT code, symbol FROM currencies ORDER BY code;

-- name: UpdateCurrency :execrows
UPDATE currencies SET symbol = ? WHERE code = ?;

-- name: DeleteCurrency :execrows
DELETE FROM currencies WHERE code = ?;

-- Amounts in the currency anywhere: incomes, expenses, accounts, balances, budgets and rates.
-- name: CurrencyInUse :one
SELECT EXISTS (SELECT 1 FROM incomes WHERE incomes.currency = sqlc.arg(code))
    OR EXISTS (SELECT 1 FROM expenses WHERE expenses.currency = sqlc.arg(code))
    OR EXISTS (SELECT 1 FROM accounts WHERE accounts.currency = sqlc.arg(code))
    OR EXISTS (SELECT 1 FROM balances WHERE balances.currency = sqlc.arg(code))
    OR EXISTS (SELECT 1 FROM budgets WHERE budgets.currency = sqlc.arg(code))
    OR EXISTS (SELECT 1 FROM exchanges WHERE exchanges.from_currency = sqlc.arg(code) OR exchanges.to_currency = sqlc.arg(code))
    OR EXISTS (SELECT 1 FROM rates WHERE rates.from_currency = sqlc.arg(code) OR rates.to_currency = sqlc.arg(code))
    AS in_use;
