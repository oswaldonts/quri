-- Rates go from 4 to 8 decimal places: rate_scaled changes from 10^4 to 10^8,
-- so every stored value is multiplied by 10^4 (3.25: 32500 → 325000000).

-- +goose Up
UPDATE rates     SET rate_scaled = rate_scaled * 10000;
UPDATE exchanges SET rate_scaled = rate_scaled * 10000;

-- +goose Down
UPDATE rates     SET rate_scaled = rate_scaled / 10000;
UPDATE exchanges SET rate_scaled = rate_scaled / 10000;
