-- Categories each period uses. A period with rows here has its own configuration;
-- a period without rows inherits from the latest earlier configured period.

-- +goose Up
CREATE TABLE period_categories (
    period      TEXT NOT NULL,
    category_id TEXT NOT NULL REFERENCES categories (id),
    PRIMARY KEY (period, category_id)
);

-- +goose Down
DROP TABLE period_categories;
