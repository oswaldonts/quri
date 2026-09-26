-- Expenses by period and category. exchange_id links the expense created in the
-- system category "exchange" for the source side of a currency exchange.

-- +goose Up
CREATE TABLE expenses (
    id           TEXT    PRIMARY KEY,
    period       TEXT    NOT NULL,
    category_id  TEXT    NOT NULL REFERENCES categories (id),
    currency     TEXT    NOT NULL,
    amount_minor INTEGER NOT NULL CHECK (amount_minor > 0),
    description  TEXT    NOT NULL DEFAULT '',
    spent_on     TEXT    NOT NULL,
    exchange_id  TEXT    UNIQUE REFERENCES exchanges (id),
    created_at   TEXT    NOT NULL
);

CREATE INDEX expenses_period_category_idx ON expenses (period, category_id);

-- Backfill: exchanges registered before this migration get their expense too.
INSERT INTO expenses (id, period, category_id, currency, amount_minor, description, spent_on, exchange_id, created_at)
SELECT 'exp-' || id, period, 'exchange', from_currency, from_amount_minor,
       'Exchange (backfilled)', exchanged_on, id, created_at
FROM exchanges;

-- +goose Down
DROP TABLE expenses;
