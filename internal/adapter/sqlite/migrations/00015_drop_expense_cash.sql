-- Expenses paid in cash are ordinary expenses now: the account (cash or bank) says where the
-- money came from, so the redundant flag goes away.

-- +goose Up
ALTER TABLE expenses DROP COLUMN cash;

-- +goose Down
ALTER TABLE expenses ADD COLUMN cash INTEGER NOT NULL DEFAULT 0;
UPDATE expenses SET cash = 1
WHERE account_id IN (SELECT id FROM accounts WHERE kind = 'cash');
