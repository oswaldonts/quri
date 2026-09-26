-- Initial schema: incomes, period rates and exchanges.
--
-- Conventions:
--   * money is stored as INTEGER minor units (cents) next to its ISO 4217 currency
--   * rates are stored as INTEGER scaled by 10^4 (3.25 → 32500)
--   * periods are TEXT 'YYYY-MM', dates TEXT 'YYYY-MM-DD', timestamps TEXT RFC 3339 (UTC)

-- +goose Up
CREATE TABLE incomes (
    id           TEXT    PRIMARY KEY,
    period       TEXT    NOT NULL,
    currency     TEXT    NOT NULL,
    amount_minor INTEGER NOT NULL CHECK (amount_minor > 0),
    kind         TEXT    NOT NULL CHECK (kind IN ('salary', 'extra', 'exchange')),
    description  TEXT    NOT NULL DEFAULT '',
    received_on  TEXT    NOT NULL,
    created_at   TEXT    NOT NULL
);

CREATE INDEX incomes_period_idx ON incomes (period);

CREATE TABLE rates (
    period        TEXT    NOT NULL,
    from_currency TEXT    NOT NULL,
    to_currency   TEXT    NOT NULL,
    rate_scaled   INTEGER NOT NULL CHECK (rate_scaled > 0),
    updated_at    TEXT    NOT NULL,
    PRIMARY KEY (period, from_currency, to_currency)
);

CREATE TABLE exchanges (
    id                TEXT    PRIMARY KEY,
    period            TEXT    NOT NULL,
    from_currency     TEXT    NOT NULL,
    from_amount_minor INTEGER NOT NULL CHECK (from_amount_minor > 0),
    to_currency       TEXT    NOT NULL,
    to_amount_minor   INTEGER NOT NULL CHECK (to_amount_minor > 0),
    rate_scaled       INTEGER NOT NULL CHECK (rate_scaled > 0),
    description       TEXT    NOT NULL DEFAULT '',
    exchanged_on      TEXT    NOT NULL,
    income_id         TEXT    NOT NULL UNIQUE REFERENCES incomes (id),
    created_at        TEXT    NOT NULL
);

CREATE INDEX exchanges_period_idx ON exchanges (period);

-- +goose Down
DROP TABLE exchanges;
DROP TABLE rates;
DROP TABLE incomes;
