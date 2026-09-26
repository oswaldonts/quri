-- Global category catalog, seeded with the initial categories.
--
-- name_key is the lowercased, trimmed name (computed in Go) so "Mercado" and
-- "mercado" can't coexist. archived_at is NULL for active categories.

-- +goose Up
CREATE TABLE categories (
    id          TEXT    PRIMARY KEY,
    name        TEXT    NOT NULL,
    name_key    TEXT    NOT NULL UNIQUE,
    group_key   TEXT    NOT NULL CHECK (group_key IN
                    ('needs', 'lifestyle', 'growth', 'obligations', 'support', 'other', 'system')),
    committed   INTEGER NOT NULL DEFAULT 0 CHECK (committed IN (0, 1)),
    archived_at TEXT,
    created_at  TEXT    NOT NULL
);

INSERT INTO categories (id, name, name_key, group_key, committed, created_at) VALUES
    ('market',           'Mercado',                  'mercado',                  'needs',       0, '2026-09-26T00:00:00Z'),
    ('home',             'Hogar',                    'hogar',                    'needs',       0, '2026-09-26T00:00:00Z'),
    ('utilities',        'Servicios',                'servicios',                'needs',       1, '2026-09-26T00:00:00Z'),
    ('personal-care',    'Cuidado personal',         'cuidado personal',         'needs',       0, '2026-09-26T00:00:00Z'),
    ('health',           'Salud',                    'salud',                    'needs',       0, '2026-09-26T00:00:00Z'),
    ('health-insurance', 'Seguro médico',            'seguro médico',            'needs',       1, '2026-09-26T00:00:00Z'),
    ('transport',        'Transporte',               'transporte',               'needs',       0, '2026-09-26T00:00:00Z'),
    ('eating-out',       'Comida fuera',             'comida fuera',             'lifestyle',   0, '2026-09-26T00:00:00Z'),
    ('leisure',          'Ocio',                     'ocio',                     'lifestyle',   0, '2026-09-26T00:00:00Z'),
    ('subscriptions',    'Suscripciones',            'suscripciones',            'lifestyle',   0, '2026-09-26T00:00:00Z'),
    ('clothing',         'Ropa',                     'ropa',                     'lifestyle',   0, '2026-09-26T00:00:00Z'),
    ('travel',           'Viajes',                   'viajes',                   'lifestyle',   0, '2026-09-26T00:00:00Z'),
    ('education',        'Educación y herramientas', 'educación y herramientas', 'growth',      0, '2026-09-26T00:00:00Z'),
    ('loans',            'Créditos',                 'créditos',                 'obligations', 1, '2026-09-26T00:00:00Z'),
    ('personal-debts',   'Deudas personales',        'deudas personales',        'obligations', 1, '2026-09-26T00:00:00Z'),
    ('family-support',   'Ayudas familiares',        'ayudas familiares',        'support',     0, '2026-09-26T00:00:00Z'),
    ('other',            'Otros',                    'otros',                    'other',       0, '2026-09-26T00:00:00Z'),
    ('exchange',         'Cambio',                   'cambio',                   'system',      0, '2026-09-26T00:00:00Z');

-- +goose Down
DROP TABLE categories;
