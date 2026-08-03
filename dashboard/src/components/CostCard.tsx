import { LucideIcon } from 'lucide-react'

interface CostCardProps {
  title: string
  value: string
  subtitle?: string
  icon: LucideIcon
  color: 'blue' | 'green' | 'amber' | 'red' | 'purple'
  trend?: number // positive = up, negative = down
}

const colorMap = {
  blue:   { bg: 'bg-blue-500/10',   icon: 'text-blue-400',   border: 'border-blue-500/20' },
  green:  { bg: 'bg-emerald-500/10', icon: 'text-emerald-400', border: 'border-emerald-500/20' },
  amber:  { bg: 'bg-amber-500/10',  icon: 'text-amber-400',  border: 'border-amber-500/20' },
  red:    { bg: 'bg-red-500/10',    icon: 'text-red-400',    border: 'border-red-500/20' },
  purple: { bg: 'bg-violet-500/10', icon: 'text-violet-400', border: 'border-violet-500/20' },
}

export function CostCard({ title, value, subtitle, icon: Icon, color, trend }: CostCardProps) {
  const c = colorMap[color]
  return (
    <div className={`rounded-xl border ${c.border} ${c.bg} p-5 flex flex-col gap-3`}>
      <div className="flex items-center justify-between">
        <span className="text-sm font-medium text-gray-400">{title}</span>
        <div className={`rounded-lg p-2 ${c.bg}`}>
          <Icon className={`h-5 w-5 ${c.icon}`} />
        </div>
      </div>
      <div>
        <p className="text-3xl font-bold text-white">{value}</p>
        {subtitle && <p className="mt-1 text-sm text-gray-500">{subtitle}</p>}
      </div>
      {trend !== undefined && (
        <div className={`text-xs font-medium ${trend >= 0 ? 'text-red-400' : 'text-emerald-400'}`}>
          {trend >= 0 ? '▲' : '▼'} {Math.abs(trend).toFixed(1)}% vs yesterday
        </div>
      )}
    </div>
  )
}
