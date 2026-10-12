-- name: GetSettings :one
SELECT spend_offset, default_currency FROM settings WHERE id = 1;

-- name: UpdateSettings :exec
UPDATE settings SET spend_offset = ?, default_currency = ? WHERE id = 1;
