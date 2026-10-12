-- A disabled expense stays registered but no longer counts: not in the summary, not in the
-- balance of its account. It can be enabled again.

-- +goose Up
ALTER TABLE expenses ADD COLUMN disabled INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE expenses DROP COLUMN disabled;
