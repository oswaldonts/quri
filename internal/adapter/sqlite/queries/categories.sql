-- name: InsertCategory :exec
INSERT INTO categories (id, name, name_key, group_key, committed, archived_at, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: GetCategory :one
SELECT id, name, name_key, group_key, committed, archived_at, created_at
FROM categories
WHERE id = ?;

-- Groups follow the dashboard order, then categories by name within each group.
-- name: ListCategories :many
SELECT id, name, name_key, group_key, committed, archived_at, created_at
FROM categories
WHERE CAST(sqlc.arg(include_archived) AS BOOLEAN) OR archived_at IS NULL
ORDER BY CASE group_key
             WHEN 'needs'       THEN 1
             WHEN 'lifestyle'   THEN 2
             WHEN 'growth'      THEN 3
             WHEN 'obligations' THEN 4
             WHEN 'support'     THEN 5
             WHEN 'other'       THEN 6
             ELSE 7
         END,
         name_key;

-- archived_at keeps its original timestamp if the category was already archived,
-- takes now when it gets archived, and is cleared when it gets unarchived.
-- name: UpdateCategory :execrows
UPDATE categories
SET name        = sqlc.arg(name),
    name_key    = sqlc.arg(name_key),
    group_key   = sqlc.arg(group_key),
    committed   = sqlc.arg(committed),
    archived_at = CASE
                      WHEN CAST(sqlc.arg(archived) AS BOOLEAN) THEN COALESCE(archived_at, sqlc.arg(now))
                      ELSE NULL
                  END
WHERE id = sqlc.arg(id);
