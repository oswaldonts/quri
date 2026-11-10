-- name: GetSettings :one
SELECT spend_offset, default_currency, closed_through FROM settings WHERE id = 1;

-- name: UpdateSettings :exec
UPDATE settings SET spend_offset = ?, default_currency = ?, closed_through = ? WHERE id = 1;
