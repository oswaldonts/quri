-- Preferences of the installation, a single row.
--
-- spend_offset: months between earning money and spending it (0: same month, 1: next month).
-- Installations that already have incomes keep the next-month behavior they were using;
-- new ones start in the same month.

-- +goose Up
CREATE TABLE settings (
    id           INTEGER PRIMARY KEY CHECK (id = 1),
    spend_offset INTEGER NOT NULL CHECK (spend_offset IN (0, 1))
);

INSERT INTO settings (id, spend_offset)
VALUES (1, CASE WHEN EXISTS (SELECT 1 FROM incomes) THEN 1 ELSE 0 END);

-- +goose Down
DROP TABLE settings;
