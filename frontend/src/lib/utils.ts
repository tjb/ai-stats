// ROI calculations — mirrors lib/roi.ts from the Next.js app
export const MINUTES_PER_ACCEPTED_LINE = 2

export function calculateROI(linesAccepted: number, hourlyRate = 150) {
  const minutesSaved = linesAccepted * MINUTES_PER_ACCEPTED_LINE
  const hoursSaved = minutesSaved / 60
  const dollarValueSaved = hoursSaved * hourlyRate
  return { minutesSaved, hoursSaved, dollarValueSaved }
}

export function formatCurrency(amount: number): string {
  return new Intl.NumberFormat('en-US', {
    style: 'currency',
    currency: 'USD',
    minimumFractionDigits: 0,
    maximumFractionDigits: 0,
  }).format(amount)
}

export function formatHours(hours: number): string {
  if (hours < 1) return `${Math.round(hours * 60)}m`
  return `${hours.toFixed(1)}h`
}

export function cn(...classes: (string | undefined | false | null)[]): string {
  return classes.filter(Boolean).join(' ')
}
