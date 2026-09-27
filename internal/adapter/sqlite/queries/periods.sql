-- The first period with any income, expense or real balance: where carry-over starts.
-- name: EarliestPeriod :one
SELECT CAST(COALESCE(MIN(period), '') AS TEXT) AS period
FROM (
    SELECT period FROM incomes
    UNION ALL
    SELECT period FROM expenses
    UNION ALL
    SELECT period FROM balances
);
