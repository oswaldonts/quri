-- Cash: an expense can be paid in cash. Those expenses stay out of the period's summary
-- (the available money is what is in the banks) and lower the cash on hand instead.
--
-- Accounts: where the money is. A bank account holds an amount typed by hand, to compare with
-- what the bank shows. The cash account holds the amount typed when the cash was counted;
-- updated_at is that moment, and the cash expenses registered after it are subtracted.

-- +goose Up
ALTER TABLE expenses ADD COLUMN cash INTEGER NOT NULL DEFAULT 0;

CREATE TABLE accounts (
    id           TEXT    PRIMARY KEY,
    name         TEXT    NOT NULL,
    kind         TEXT    NOT NULL CHECK (kind IN ('bank', 'cash')),
    currency     TEXT    NOT NULL,
    amount_minor INTEGER NOT NULL,
    updated_at   TEXT    NOT NULL,
    UNIQUE (name, currency)
);

CREATE UNIQUE INDEX accounts_cash_currency_idx ON accounts (currency) WHERE kind = 'cash';

-- +goose Down
DROP TABLE accounts;
ALTER TABLE expenses DROP COLUMN cash;
