-- Money moves between accounts. A transfer takes an amount out of one account and puts it in
-- another of the same currency; it is not an expense nor an income, so no summary counts it.
-- An exchange can also say which account the source money comes out of and which one receives the
-- converted amount. The balance of an account is what was typed by hand plus these movements and
-- minus its expenses, all registered after that.

-- +goose Up
ALTER TABLE exchanges ADD COLUMN from_account_id TEXT;
ALTER TABLE exchanges ADD COLUMN to_account_id TEXT;

CREATE TABLE transfers (
    id              TEXT    PRIMARY KEY,
    from_account_id TEXT    NOT NULL REFERENCES accounts (id),
    to_account_id   TEXT    NOT NULL REFERENCES accounts (id),
    currency        TEXT    NOT NULL,
    amount_minor    INTEGER NOT NULL CHECK (amount_minor > 0),
    description     TEXT    NOT NULL DEFAULT '',
    transferred_on  TEXT    NOT NULL,
    created_at      TEXT    NOT NULL,
    CHECK (from_account_id <> to_account_id)
);

CREATE INDEX transfers_from_idx ON transfers (from_account_id);
CREATE INDEX transfers_to_idx ON transfers (to_account_id);

-- +goose Down
DROP INDEX transfers_to_idx;
DROP INDEX transfers_from_idx;
DROP TABLE transfers;
ALTER TABLE exchanges DROP COLUMN to_account_id;
ALTER TABLE exchanges DROP COLUMN from_account_id;
