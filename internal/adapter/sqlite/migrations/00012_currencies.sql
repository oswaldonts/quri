-- Currencies: the list is managed by the user, like the accounts. Amounts in other tables keep
-- the plain code; the service checks it is registered before accepting a new amount.
-- The seed is only a starting point; any of them can be removed while unused. Symbols shared by
-- several currencies ($) are left empty so those keep their code on screen.

-- +goose Up
CREATE TABLE currencies (
    code   TEXT PRIMARY KEY,
    symbol TEXT NOT NULL DEFAULT ''
);

INSERT INTO currencies (code, symbol) VALUES
    ('ARS', ''),
    ('COP', ''),
    ('EUR', '€'),
    ('PEN', 'S/'),
    ('USD', '$'),
    ('VES', '');

-- +goose Down
DROP TABLE currencies;
