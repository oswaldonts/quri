const pad = (n: number) => String(n).padStart(2, '0')

// toPeriod formats a date as YYYY-MM in local time (toISOString would use UTC and
// report the wrong month in the last hours of the month).
export function toPeriod(date: Date): string {
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}`
}

// shift moves a YYYY-MM period by a number of months.
export function shift(period: string, months: number): string {
  const [year, month] = period.split('-').map(Number)
  return toPeriod(new Date(year, month - 1 + months, 1))
}

// today is the current local date as YYYY-MM-DD.
export function today(): string {
  const now = new Date()
  return `${toPeriod(now)}-${pad(now.getDate())}`
}

const monthFormat = new Intl.DateTimeFormat('es', { month: 'long', year: 'numeric' })

// monthName writes a YYYY-MM period as "septiembre de 2026".
export function monthName(period: string): string {
  const [year, month] = period.split('-').map(Number)
  return monthFormat.format(new Date(year, month - 1, 1))
}
