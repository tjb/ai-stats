import type { TooltipValueType } from 'recharts'

// Recharts 3 hands tooltip formatters a value typed as number | string |
// readonly (number | string)[] | undefined. Our charts plot single numeric
// series, so these helpers narrow that down to a number before formatting.

/** Formats a tooltip value with `format`, tolerating missing, string and range values. */
export function formatTooltipValue(
  value: TooltipValueType | undefined,
  format: (n: number) => string,
): string {
  if (value === undefined) return '—'
  if (typeof value === 'number') return format(value)
  if (typeof value === 'string') {
    const n = Number(value)
    return value.trim() !== '' && Number.isFinite(n) ? format(n) : value
  }
  return value.map((v) => formatTooltipValue(v, format)).join(' – ')
}

/** Builds a Tooltip `formatter` that renders a single series as [format(value), label]. */
export function tooltipFormatter(format: (n: number) => string, label: string) {
  return (value: TooltipValueType | undefined): [string, string] => [
    formatTooltipValue(value, format),
    label,
  ]
}
