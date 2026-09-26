-- name: ListPeriodCategories :many
SELECT sqlc.embed(categories)
FROM categories
JOIN period_categories ON period_categories.category_id = categories.id
WHERE period_categories.period = ?
ORDER BY CASE categories.group_key
             WHEN 'needs'       THEN 1
             WHEN 'lifestyle'   THEN 2
             WHEN 'growth'      THEN 3
             WHEN 'obligations' THEN 4
             WHEN 'support'     THEN 5
             WHEN 'other'       THEN 6
             ELSE 7
         END,
         categories.name_key;

-- Periods are 'YYYY-MM' text, so string order is chronological order.
-- name: LatestConfiguredPeriodBefore :one
SELECT period
FROM period_categories
WHERE period < ?
ORDER BY period DESC
LIMIT 1;

-- name: DeletePeriodCategories :exec
DELETE FROM period_categories WHERE period = ?;

-- name: InsertPeriodCategory :exec
INSERT INTO period_categories (period, category_id) VALUES (?, ?);
