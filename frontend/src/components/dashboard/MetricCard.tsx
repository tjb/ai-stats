import type { ReactNode } from 'react'
import { TrendingUp, TrendingDown, Minus } from 'lucide-react'
import { cn } from '../../lib/utils'

interface MetricCardProps {
  label: string
  value: string
  change?: number
  changeLabel?: string
  description?: ReactNode
  className?: string
}

export function MetricCard({
  label,
  value,
  change,
  changeLabel,
  description,
  className,
}: MetricCardProps) {
  const isPositive = change !== undefined && change > 0
  const isNegative = change !== undefined && change < 0

  return (
    <div className={cn('rounded-xl border border-slate-800 bg-slate-900 p-6', className)}>
      <p className="text-sm font-medium text-slate-400">{label}</p>
      <p className="mt-2 text-3xl font-bold text-white">{value}</p>
      {change !== undefined && (
        <div className="mt-2 flex items-center gap-1.5">
          {isPositive && <TrendingUp className="h-3.5 w-3.5 text-emerald-400" />}
          {isNegative && <TrendingDown className="h-3.5 w-3.5 text-red-400" />}
          {!isPositive && !isNegative && <Minus className="h-3.5 w-3.5 text-slate-500" />}
          <span
            className={cn(
              'text-xs font-medium',
              isPositive && 'text-emerald-400',
              isNegative && 'text-red-400',
              !isPositive && !isNegative && 'text-slate-500',
            )}
          >
            {isPositive && '+'}
            {change}% {changeLabel ?? 'vs last month'}
          </span>
        </div>
      )}
      {description && <p className="mt-1 text-xs text-slate-500">{description}</p>}
    </div>
  )
}
