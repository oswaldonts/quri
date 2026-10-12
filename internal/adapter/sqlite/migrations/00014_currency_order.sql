-- The user picks the order currencies are listed in (sheets, selectors) and which one opens by
-- default. Existing currencies start in code order.

-- +goose Up
ALTER TABLE currencies ADD COLUMN position INTEGER NOT NULL DEFAULT 0;
UPDATE currencies SET position = (SELECT COUNT(*) FROM currencies AS other WHERE other.code < currencies.code);

-- Removing the default currency's row clears the setting instead of leaving a dangling code.
ALTER TABLE settings ADD COLUMN default_currency TEXT REFERENCES currencies (code) ON DELETE SET NULL;

-- +goose Down
CREATE TABLE settings_old (
    id           INTEGER PRIMARY KEY CHECK (id = 1),
    spend_offset INTEGER NOT NULL CHECK (spend_offset IN (0, 1))
);
INSERT INTO settings_old (id, spend_offset) SELECT id, spend_offset FROM settings;
DROP TABLE settings;
ALTER TABLE settings_old RENAME TO settings;
ALTER TABLE currencies DROP COLUMN position;
