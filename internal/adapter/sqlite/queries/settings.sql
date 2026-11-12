-- name: GetSettings :one
SELECT default_currency, closed_through FROM settings WHERE id = 1;

-- name: UpdateSettings :exec
UPDATE settings SET default_currency = ?, closed_through = ? WHERE id = 1;
