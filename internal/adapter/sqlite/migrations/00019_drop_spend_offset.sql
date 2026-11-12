-- The money of a month is no longer said to be spent the month after: the period is the month
-- the entries belong to, so the spend offset setting goes away.

-- +goose Up
ALTER TABLE settings DROP COLUMN spend_offset;

-- +goose Down
ALTER TABLE settings ADD COLUMN spend_offset INTEGER NOT NULL DEFAULT 0 CHECK (spend_offset IN (0, 1));
