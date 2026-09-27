#!/usr/bin/env bash
# curl examples for every endpoint, using the period 2026-08 (money earned in August,
# spent in September). Each section can be copied on its own; run as a whole it goes
# through a full month and then edits and deletes some of what it created.
#
#   API=http://localhost:8080 docs/api-examples.sh
#
# Requires curl and jq. IDs returned by the API are captured into variables.
set -euo pipefail

API="${API:-http://localhost:8080}"
P="2026-08"
JSON=(-H "Content-Type: application/json")
# --fail-with-body stops the script on a 4xx/5xx and still prints the problem details.
CURL=(curl -sS --fail-with-body)

section() { printf '\n=== %s\n' "$*"; }

section "System"
"${CURL[@]}" "$API/health"
"${CURL[@]}" "$API/openapi.yaml" -o /dev/null -w 'openapi.yaml: %{http_code}\n'
echo "Docs (Swagger UI): $API/docs"

section "Categories: catalog"
"${CURL[@]}" "$API/categories" | jq -r '.[] | "\(.id)\t\(.name)"'
"${CURL[@]}" "$API/categories?include_archived=true" | jq length
PETS=$("${CURL[@]}" -X POST "$API/categories" "${JSON[@]}" \
  -d '{"name": "Mascotas", "group": "needs", "committed": false}' | jq -r .id)
"${CURL[@]}" -X PATCH "$API/categories/$PETS" "${JSON[@]}" -d '{"committed": true}' | jq -c .

section "Categories: per period"
"${CURL[@]}" "$API/periods/$P/categories" | jq -c '{source, inherited_from, count: (.categories | length)}'
"${CURL[@]}" -X PUT "$API/periods/$P/categories/$PETS" | jq -c '{source, count: (.categories | length)}'
"${CURL[@]}" -X DELETE "$API/periods/$P/categories/travel" | jq -c '{source, count: (.categories | length)}'

section "Rates"
"${CURL[@]}" -X PUT "$API/periods/$P/rates/USD-PEN" "${JSON[@]}" -d '{"rate": "3.25"}' | jq -c .
"${CURL[@]}" "$API/periods/$P/rates/USD-PEN" | jq -c .

section "Incomes"
SALARY=$("${CURL[@]}" -X POST "$API/periods/$P/incomes" "${JSON[@]}" \
  -d '{"currency": "USD", "amount": "1330.00", "kind": "salary", "description": "Sueldo agosto", "received_on": "2026-08-31"}' | jq -r .id)
EXTRA=$("${CURL[@]}" -X POST "$API/periods/$P/incomes" "${JSON[@]}" \
  -d '{"currency": "PEN", "amount": "108.00", "kind": "extra", "category_id": "loans", "description": "Ganancia del cambio", "received_on": "2026-09-01"}' | jq -r .id)
"${CURL[@]}" "$API/periods/$P/incomes" | jq -c '.[] | {kind, currency, amount, category_id}'
"${CURL[@]}" -X PATCH "$API/periods/$P/incomes/$SALARY" "${JSON[@]}" -d '{"amount": "1350.00"}' | jq -c '{amount}'

section "Exchanges (create an expense in Cambio and an income in the target currency)"
EXCHANGE=$("${CURL[@]}" -X POST "$API/periods/$P/exchanges" "${JSON[@]}" \
  -d '{"from": "USD", "to": "PEN", "amount": "1096.00", "description": "Casa de cambio", "exchanged_on": "2026-09-01"}' | jq -r .id)
"${CURL[@]}" -X POST "$API/periods/$P/exchanges" "${JSON[@]}" \
  -d '{"from": "USD", "to": "PEN", "amount": "100.00", "exchanged_on": "2026-09-15"}' | jq -c '{from, to, rate}'
"${CURL[@]}" "$API/periods/$P/exchanges" | jq -c '.[] | {id, from: .from.amount, to: .to.amount}'

section "Expenses"
INSURANCE=$("${CURL[@]}" -X POST "$API/periods/$P/expenses" "${JSON[@]}" \
  -d '{"category_id": "health-insurance", "currency": "PEN", "amount": "1000.00", "description": "Seguro médico", "spent_on": "2026-09-02"}' | jq -r .id)
"${CURL[@]}" -X POST "$API/periods/$P/expenses/batch" "${JSON[@]}" -d '{
  "category_id": "market",
  "currency": "PEN",
  "spent_on": "2026-09-03",
  "tags": ["mercado"],
  "items": [
    {"description": "Cereal + Avena + Leche", "amount": "80"},
    {"description": "Huevos", "amount": "9"},
    {"description": "Queso", "amount": "37", "tags": ["lácteos"]},
    {"description": "Chocolate Milky", "amount": "27", "tags": ["dulces"]},
    {"description": "Pringles", "amount": "8.5", "tags": ["dulces", "snacks"]},
    {"description": "Cine", "amount": "20", "category_id": "leisure", "tags": ["salidas"]}
  ]
}' | jq -c 'length'
"${CURL[@]}" "$API/periods/$P/expenses" | jq -c '.[] | {category_id, currency, amount, description, tags}'
"${CURL[@]}" "$API/periods/$P/expenses?category_id=market" | jq length
"${CURL[@]}" "$API/periods/$P/expenses?tag=dulces" | jq -c '[.[].description]'
"${CURL[@]}" -X PATCH "$API/periods/$P/expenses/$INSURANCE" "${JSON[@]}" \
  -d '{"amount": "1050.00", "tags": ["salud"]}' | jq -c '{amount, tags}'

section "Budgets"
"${CURL[@]}" -X PUT "$API/periods/$P/budgets/market/PEN" "${JSON[@]}" -d '{"amount": "150"}' | jq -c .
"${CURL[@]}" -X PUT "$API/periods/$P/budgets/leisure/PEN" "${JSON[@]}" -d '{"amount": "50"}' | jq -c .
"${CURL[@]}" "$API/periods/$P/budgets" | jq -c '.[] | {category_id, currency, amount}'

section "Real balance"
"${CURL[@]}" -X PUT "$API/periods/$P/balances/PEN" "${JSON[@]}" -d '{"amount": "2450.00"}' | jq -c .
"${CURL[@]}" -X PUT "$API/periods/$P/balances/USD" "${JSON[@]}" -d '{"amount": "154.00"}' | jq -c .
"${CURL[@]}" "$API/periods/$P/balances" | jq -c '.[] | {currency, amount}'

section "Available: how much is left"
"${CURL[@]}" "$API/periods/$P/available" | jq -c '.[]'

section "Summary"
"${CURL[@]}" "$API/periods/$P/summary" | jq '.currencies[] | {
  currency, income: .income.total, spent, available, real, difference, budgeted,
  contributions: [.contributions[] | "\(.name): \(.amount)"],
  categories: [.categories[] | "\(.name): \(.spent)\(if .budget then " / \(.budget) (queda \(.remaining))" else "" end)"],
  tags: [.tags[] | "\(.tag): \(.spent)"],
  top_items: [.top_items[] | "\(.description): \(.spent)"]
}'

section "Summary of one currency"
"${CURL[@]}" "$API/periods/$P/summary/PEN" | jq -c '{currency, available, contributions: [.contributions[] | "\(.name): \(.amount)"]}'

section "Trends"
"${CURL[@]}" "$API/trends?from=2026-06&to=$P&currency=PEN" | jq -c '{
  periods: [.periods[] | "\(.period): \(.spent)"],
  categories: [.categories[] | "\(.name): total \(.total), average \(.average)"]
}'

section "Planning: planned entries and the projected view"
PLANNED=$("${CURL[@]}" -X POST "$API/periods/$P/expenses" "${JSON[@]}" \
  -d '{"category_id": "utilities", "currency": "PEN", "amount": "120.00", "description": "Luz (estimado)", "spent_on": "2026-09-20", "status": "planned"}' | jq -r .id)
"${CURL[@]}" "$API/periods/$P/expenses?status=planned" | jq -c '[.[] | {description, amount, status}]'
"${CURL[@]}" "$API/periods/$P/available" | jq -c '[.[] | {currency, available}]'                 # confirmed only
"${CURL[@]}" "$API/periods/$P/available?view=projected" | jq -c '[.[] | {currency, available}]'  # with the plan
# It happened, for a slightly different amount: confirm and correct in one request.
"${CURL[@]}" -X PATCH "$API/periods/$P/expenses/$PLANNED" "${JSON[@]}" \
  -d '{"status": "confirmed", "amount": "126.40"}' | jq -c '{description, amount, status}'

section "Deletes"
"${CURL[@]}" -X DELETE "$API/periods/$P/budgets/leisure/PEN" -o /dev/null -w 'budget: %{http_code}\n'
"${CURL[@]}" -X DELETE "$API/periods/$P/balances/USD" -o /dev/null -w 'balance: %{http_code}\n'
"${CURL[@]}" -X DELETE "$API/periods/$P/expenses/$INSURANCE" -o /dev/null -w 'expense: %{http_code}\n'
"${CURL[@]}" -X DELETE "$API/periods/$P/incomes/$EXTRA" -o /dev/null -w 'income: %{http_code}\n'
"${CURL[@]}" -X DELETE "$API/periods/$P/exchanges/$EXCHANGE" -o /dev/null -w 'exchange (with its income and expense): %{http_code}\n'
# Categories are never deleted: archive them. Mascotas leaves the period first.
"${CURL[@]}" -X DELETE "$API/periods/$P/categories/$PETS" -o /dev/null -w 'period category: %{http_code}\n'
"${CURL[@]}" -X PATCH "$API/categories/$PETS" "${JSON[@]}" -d '{"archived": true}' | jq -c '{name, archived}'

section "Expected errors (problem+json)"
curl -sS -X POST "$API/periods/$P/incomes" "${JSON[@]}" \
  -d '{"currency": "$", "amount": "10", "kind": "salary", "received_on": "2026-08-31"}' | jq -c '{status, detail, errors}'
curl -sS -X POST "$API/periods/$P/exchanges" "${JSON[@]}" \
  -d '{"from": "USD", "to": "ARS", "amount": "10", "exchanged_on": "2026-09-01"}' | jq -c '{status, detail}'
