import React from 'react'
import {
  AreaChart,
  Area,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Legend,
} from 'recharts'
import { TimeSeriesPoint } from '../types'

interface UsageChartProps {
  data: TimeSeriesPoint[]
  title: string
}

// Generate mock time-series data for the last 24 hours
export function generateMockTimeSeries(): TimeSeriesPoint[] {
  const now = Date.now()
  return Array.from({ length: 24 }, (_, i) => {
    const hour = new Date(now - (23 - i) * 3_600_000)
    const base = 80_000 + Math.random() * 50_000
    return {
      time: hour.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
      tokens: Math.floor(base),
      cost: parseFloat((base / 1000 * 0.03).toFixed(2)),
      requests: Math.floor(base / 150),
    }
  })
}

const CustomTooltip = ({ active, payload, label }: any) => {
  if (active && payload?.length) {
    return (
      <div className="rounded-lg border border-gray-700 bg-gray-900 p-3 text-sm shadow-xl">
        <p className="mb-1 font-semibold text-gray-300">{label}</p>
        {payload.map((entry: any) => (
          <p key={entry.name} style={{ color: entry.color }}>
            {entry.name}: {entry.name === 'cost' ? `$${entry.value}` : entry.value.toLocaleString()}
          </p>
        ))}
      </div>
    )
  }
  return null
}

export function UsageChart({ data, title }: UsageChartProps) {
  return (
    <div className="rounded-xl border border-gray-800 bg-gray-900/50 p-5">
      <h3 className="mb-4 text-sm font-semibold text-gray-300">{title}</h3>
      <ResponsiveContainer width="100%" height={220}>
        <AreaChart data={data} margin={{ top: 4, right: 4, left: 0, bottom: 0 }}>
          <defs>
            <linearGradient id="colorTokens" x1="0" y1="0" x2="0" y2="1">
              <stop offset="5%" stopColor="#0ea5e9" stopOpacity={0.3} />
              <stop offset="95%" stopColor="#0ea5e9" stopOpacity={0} />
            </linearGradient>
            <linearGradient id="colorCost" x1="0" y1="0" x2="0" y2="1">
              <stop offset="5%" stopColor="#10b981" stopOpacity={0.3} />
              <stop offset="95%" stopColor="#10b981" stopOpacity={0} />
            </linearGradient>
          </defs>
          <CartesianGrid strokeDasharray="3 3" stroke="#1f2937" />
          <XAxis
            dataKey="time"
            tick={{ fill: '#6b7280', fontSize: 11 }}
            tickLine={false}
            axisLine={false}
          />
          <YAxis
            tick={{ fill: '#6b7280', fontSize: 11 }}
            tickLine={false}
            axisLine={false}
            tickFormatter={(v) => v >= 1000 ? `${(v / 1000).toFixed(0)}K` : v}
          />
          <Tooltip content={<CustomTooltip />} />
          <Legend
            wrapperStyle={{ fontSize: 12, color: '#9ca3af' }}
          />
          <Area
            type="monotone"
            dataKey="tokens"
            name="tokens"
            stroke="#0ea5e9"
            strokeWidth={2}
            fill="url(#colorTokens)"
          />
          <Area
            type="monotone"
            dataKey="requests"
            name="requests"
            stroke="#8b5cf6"
            strokeWidth={2}
            fill="none"
          />
        </AreaChart>
      </ResponsiveContainer>
    </div>
  )
}
