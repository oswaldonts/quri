-- "Closing the month": the user has finished a period and moves on to the next one before the
-- calendar gets there (e.g. the salary arrived early). The period closed last is kept here.

-- +goose Up
ALTER TABLE settings ADD COLUMN closed_through TEXT;

-- +goose Down
ALTER TABLE settings DROP COLUMN closed_through;
