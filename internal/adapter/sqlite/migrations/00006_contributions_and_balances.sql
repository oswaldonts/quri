-- Contributions ("aportes"): an extra income may be assigned to a category.
-- Balances: the real money per currency at the end of a period.

-- +goose Up
ALTER TABLE incomes ADD COLUMN category_id TEXT REFERENCES categories (id);

CREATE INDEX incomes_period_category_idx ON incomes (period, category_id);

CREATE TABLE balances (
    period       TEXT    NOT NULL,
    currency     TEXT    NOT NULL,
    amount_minor INTEGER NOT NULL,
    updated_at   TEXT    NOT NULL,
    PRIMARY KEY (period, currency)
);

-- +goose Down
DROP TABLE balances;
DROP INDEX incomes_period_category_idx;
ALTER TABLE incomes DROP COLUMN category_id;
