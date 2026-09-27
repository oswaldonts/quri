-- Tags describe the kind of an expense ("dulces", "lácteos"); an expense can have several.
-- Stored normalized (lowercase, trimmed) by the core.

-- +goose Up
CREATE TABLE expense_tags (
    expense_id TEXT NOT NULL REFERENCES expenses (id) ON DELETE CASCADE,
    tag        TEXT NOT NULL,
    PRIMARY KEY (expense_id, tag)
);

CREATE INDEX expense_tags_tag_idx ON expense_tags (tag);

-- +goose Down
DROP TABLE expense_tags;
