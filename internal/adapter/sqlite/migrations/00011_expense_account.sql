-- An expense is charged to an account (a bank or the cash). The account's balance is what was
-- typed by hand minus the confirmed expenses charged to it since then.
--
-- cash stays as the stored form of "the account is the cash": those expenses are left out of the
-- period summary, whose available money is what is in the banks.

-- +goose Up
ALTER TABLE expenses ADD COLUMN account_id TEXT;

UPDATE expenses
SET account_id = (SELECT accounts.id FROM accounts WHERE accounts.kind = 'cash' AND accounts.currency = expenses.currency)
WHERE cash = 1;

CREATE INDEX expenses_account_idx ON expenses (account_id) WHERE account_id IS NOT NULL;

-- +goose Down
DROP INDEX expenses_account_idx;
ALTER TABLE expenses DROP COLUMN account_id;
