-- Expenses that repeat for a number of months (insurance, loan installments, subscriptions)
-- are stored as one expense per month, linked by a series: series_no of series_total.

-- +goose Up
ALTER TABLE expenses ADD COLUMN series_id TEXT;
ALTER TABLE expenses ADD COLUMN series_no INTEGER;
ALTER TABLE expenses ADD COLUMN series_total INTEGER;
CREATE INDEX expenses_series ON expenses (series_id) WHERE series_id IS NOT NULL;

-- +goose Down
DROP INDEX expenses_series;
ALTER TABLE expenses DROP COLUMN series_total;
ALTER TABLE expenses DROP COLUMN series_no;
ALTER TABLE expenses DROP COLUMN series_id;
