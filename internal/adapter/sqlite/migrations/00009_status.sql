-- Entries can be planned (expected, not happened yet) or confirmed (happened).
-- Everything registered so far happened, so it defaults to confirmed.

-- +goose Up
ALTER TABLE incomes   ADD COLUMN status TEXT NOT NULL DEFAULT 'confirmed' CHECK (status IN ('planned', 'confirmed'));
ALTER TABLE expenses  ADD COLUMN status TEXT NOT NULL DEFAULT 'confirmed' CHECK (status IN ('planned', 'confirmed'));
ALTER TABLE exchanges ADD COLUMN status TEXT NOT NULL DEFAULT 'confirmed' CHECK (status IN ('planned', 'confirmed'));

-- +goose Down
ALTER TABLE exchanges DROP COLUMN status;
ALTER TABLE expenses  DROP COLUMN status;
ALTER TABLE incomes   DROP COLUMN status;
