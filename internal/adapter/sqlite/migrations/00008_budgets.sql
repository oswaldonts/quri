-- Planned amount per category, period and currency.

-- +goose Up
CREATE TABLE budgets (
    period       TEXT    NOT NULL,
    category_id  TEXT    NOT NULL REFERENCES categories (id),
    currency     TEXT    NOT NULL,
    amount_minor INTEGER NOT NULL CHECK (amount_minor > 0),
    updated_at   TEXT    NOT NULL,
    PRIMARY KEY (period, category_id, currency)
);

-- +goose Down
DROP TABLE budgets;
