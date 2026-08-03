import { AgentStats } from '../types'
import { Shield, AlertTriangle, Clock } from 'lucide-react'

interface AgentRowProps {
  agent: AgentStats
  rank: number
  budgetCap: number // USD cents
}

function formatCost(usd: number): string {
  return `$${usd.toFixed(2)}`
}

function formatTokens(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(0)}K`
  return n.toString()
}

function timeAgo(isoDate: string): string {
  const seconds = Math.floor((Date.now() - new Date(isoDate).getTime()) / 1000)
  if (seconds < 60) return `${seconds}s ago`
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`
  if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`
  return `${Math.floor(seconds / 86400)}d ago`
}

export function AgentRow({ agent, rank, budgetCap }: AgentRowProps) {
  const budgetUsedPct = Math.min((agent.total_cost / (budgetCap / 100)) * 100, 100)
  const isWarning = budgetUsedPct > 75
  const isCritical = budgetUsedPct > 90

  const barColor = isCritical
    ? 'bg-red-500'
    : isWarning
    ? 'bg-amber-500'
    : 'bg-emerald-500'

  return (
    <tr className="border-b border-gray-800 hover:bg-gray-800/40 transition-colors">
      {/* Rank */}
      <td className="py-3 px-4 text-sm text-gray-500 w-10">#{rank}</td>

      {/* Agent ID */}
      <td className="py-3 px-4">
        <div className="flex items-center gap-2">
          <div className="w-2 h-2 rounded-full bg-emerald-400 animate-pulse" />
          <span className="font-mono text-sm text-white">{agent.agent_id}</span>
          {agent.redactions_triggered > 0 && (
            <span className="flex items-center gap-1 rounded-full bg-amber-500/10 px-2 py-0.5 text-xs text-amber-400 border border-amber-500/20">
              <Shield className="h-3 w-3" />
              {agent.redactions_triggered} redacted
            </span>
          )}
        </div>
      </td>

      {/* Token Usage */}
      <td className="py-3 px-4 text-sm text-gray-300">
        {formatTokens(agent.total_tokens)}
      </td>

      {/* Cost */}
      <td className="py-3 px-4 text-sm font-semibold text-white">
        {formatCost(agent.total_cost)}
      </td>

      {/* Budget Bar */}
      <td className="py-3 px-4 w-40">
        <div className="flex items-center gap-2">
          <div className="flex-1 h-1.5 rounded-full bg-gray-700">
            <div
              className={`h-1.5 rounded-full ${barColor} transition-all duration-500`}
              style={{ width: `${budgetUsedPct}%` }}
            />
          </div>
          <span className={`text-xs font-medium ${isCritical ? 'text-red-400' : 'text-gray-400'}`}>
            {budgetUsedPct.toFixed(0)}%
          </span>
          {isCritical && <AlertTriangle className="h-3.5 w-3.5 text-red-400" />}
        </div>
      </td>

      {/* Requests */}
      <td className="py-3 px-4 text-sm text-gray-400">
        {agent.request_count.toLocaleString()}
      </td>

      {/* Last Active */}
      <td className="py-3 px-4 text-sm text-gray-500">
        <div className="flex items-center gap-1">
          <Clock className="h-3.5 w-3.5" />
          {timeAgo(agent.last_request_at)}
        </div>
      </td>
    </tr>
  )
}
