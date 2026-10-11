import type { Schemas } from './api'

// Amounts travel as decimal strings ("1330.00") so no precision is lost to floating point.
// Intl.NumberFormat formats a numeric string exactly, so there is no Number() round trip.
const number = new Intl.NumberFormat(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })

// Currencies whose symbol is unambiguous here. ARS, COP and VES would all be "$", so they
// keep their code.
const symbols: Record<string, string> = { USD: '$', PEN: 'S/ ' }

export function formatMoney(amount: string, currency: string): string {
  const negative = amount.startsWith('-') && /[1-9]/.test(amount)
  const digits = negative ? amount.slice(1) : amount
  const symbol = symbols[currency]
  let formatted: string
  try {
    formatted = number.format(digits as Intl.StringNumericLiteral)
  } catch {
    // An engine without string support: show the digits as the API sent them.
    formatted = digits
  }
  const body = symbol ? `${symbol}${formatted}` : `${currency} ${formatted}`
  return negative ? `-${body}` : body
}

// isNegative tells whether a decimal string is below zero, without parsing it as a number.
export function isNegative(amount: string): boolean {
  return amount.startsWith('-') && /[1-9]/.test(amount)
}

export type Currency = Schemas['RegisterIncomeInputBody']['currency']

// The compiler checks every entry against the API's enum; a currency the API adds
// is missing here until it is listed.
export const currencies: Currency[] = ['ARS', 'COP', 'PEN', 'USD', 'VES']

const amountPattern = /^[0-9]{1,13}(\.[0-9]{1,2})?$/

// parseAmount accepts what people type ("1 330,50") and returns the decimal string the API
// expects ("1330.50"), or null when it isn't a valid amount.
export function parseAmount(input: string): string | null {
  const amount = input.replace(/\s/g, '').replace(',', '.')
  return amountPattern.test(amount) ? amount : null
}

const signedPattern = /^-?[0-9]{1,13}(\.[0-9]{1,2})?$/

// parseSigned is parseAmount for balances, which can be negative (a card debt).
export function parseSigned(input: string): string | null {
  const amount = input.replace(/\s/g, '').replace(',', '.')
  return signedPattern.test(amount) ? amount : null
}

// isZero tells whether a decimal string is zero ("0", "0.00", "-0.00").
export function isZero(amount: string): boolean {
  return !/[1-9]/.test(amount)
}

const rateRe = /^[0-9]{1,10}(\.[0-9]{1,8})?$/

// parseRate is parseAmount for exchange rates (up to 8 decimals).
export function parseRate(input: string): string | null {
  const rate = input.replace(/\s/g, '').replace(',', '.')
  return rateRe.test(rate) && /[1-9]/.test(rate) ? rate : null
}

// Amounts have exactly 2 decimals, so they are summed as integer cents (BigInt): no float error.
function toCents(amount: string): bigint {
  const [i, f = ''] = amount.split('.')
  return BigInt(i + f.padEnd(2, '0'))
}

function fromCents(cents: bigint): string {
  const digits = cents.toString().padStart(3, '0')
  return `${digits.slice(0, -2)}.${digits.slice(-2)}`
}

export function sumAmounts(amounts: string[]): string {
  return fromCents(amounts.reduce((total, a) => total + toCents(a), 0n))
}

export function subtractAmounts(a: string, b: string): string {
  const d = toCents(a) - toCents(b)
  return d < 0n ? `-${fromCents(-d)}` : fromCents(d)
}

// compareAmounts orders decimal strings (negative, zero or positive like a - b).
export function compareAmounts(a: string, b: string): number {
  const d = toCents(a) - toCents(b)
  return d < 0n ? -1 : d > 0n ? 1 : 0
}

// convert gives what an amount becomes at a rate: amount × rate, truncated to cents like the
// API does. It works on integers (BigInt) so the preview matches what the server will register.
export function convert(amount: string, rate: string): string {
  const [ri, rf = ''] = rate.split('.')
  const scaled = BigInt(ri + rf.padEnd(8, '0'))
  return fromCents((toCents(amount) * scaled) / 100000000n)
}
