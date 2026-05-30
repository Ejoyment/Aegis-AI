import React, { useState } from 'react'
import {
  DollarSign,
  Activity,
  Bot,
  AlertTriangle,
  RefreshCw,
  Shield,
  TrendingUp,
} from 'lucide-react'
import { CostCard } from './components/CostCard'
import { AgentRow } from './components/AgentRow'
import { UsageChart, generateMockTimeSeries } from './components/UsageChart'
import { useTotalStats, useAgentStats, useRecentLogs } from './hooks/useApi'

export default function App() {
  const { data: totals, loading: totalsLoading } = useTotalStats()
  const { data: agents, loading: agentsLoading } = useAgentStats()
  const { data: logs } = useRecentLogs(50)
  const [chartData] = useState(() => generateMockTimeSeries())

  const topCostAgent = agents.sort((a, b) => b.total_cost - a.total_cost)[0]

  return (
    <div className="min-h-screen bg-gray-950">
      {/* Header */}
      <header className="border-b border-gray-800 bg-gray-900/50 backdrop-blur-sm sticky top-0 z-10">
        <div className="mx-auto max-w-7xl px-6 py-4 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <Shield className="h-7 w-7 text-aegis-500" />
            <h1 className="text-xl font-bold text-white">AegisAI</h1>
            <span className="rounded-full bg-aegis-500/10 px-3 py-0.5 text-xs font-medium text-aegis-400 border border-aegis-500/20">
              FinOps Dashboard
            </span>
          </div>
          <div className="flex items-center gap-4 text-sm text-gray-400">
            <span className="flex items-center gap-1">
              <Activity className="h-4 w-4 text-emerald-400" />
              Live
            </span>
            <span className="flex items-center gap-1">
              <RefreshCw className="h-3.5 w-3.5" />
              Auto-refreshing 5s
            </span>
          </div>
        </div>
      </header>

      <main className="mx-auto max-w-7xl px-6 py-6 space-y-6">
        {/* Cost Cards */}
        <section className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          <CostCard
            title="Total Spend (30d)"
            value={`$${totals.total_cost.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`}
            subtitle={`${totals.agent_count} active agents`}
            icon={DollarSign}
            color="blue"
            trend={12.5}
          />
          <CostCard
            title="Tokens Consumed"
            value={totals.total_tokens >= 1_000_000
              ? `${(totals.total_tokens / 1_000_000).toFixed(1)}M`
              : `${(totals.total_tokens / 1_000).toFixed(0)}K`}
            subtitle={`${totals.total_requests.toLocaleString()} total requests`}
            icon={Activity}
            color="green"
            trend={-3.2}
          />
          <CostCard
            title="Top Spender"
            value={topCostAgent?.agent_id?.split('-').slice(0, 2).join(' ') || 'N/A'}
            subtitle={topCostAgent ? `$${topCostAgent.total_cost.toFixed(2)}` : '—'}
            icon={Bot}
            color="purple"
          />
          <CostCard
            title="Rate Limits Hit"
            value={logs.filter(l => l.status_code === 429).length.toString()}
            subtitle="recently triggered"
            icon={AlertTriangle}
            color="amber"
          />
        </section>

        {/* Usage Chart */}
        <section>
          <UsageChart data={chartData} title="Token Usage & Request Volume (24h)" />
        </section>

        {/* Agent Table */}
        <section className="rounded-xl border border-gray-800 bg-gray-900/50 overflow-hidden">
          <div className="p-5 border-b border-gray-800">
            <div className="flex items-center justify-between">
              <h3 className="text-sm font-semibold text-gray-300 flex items-center gap-2">
                <TrendingUp className="h-4 w-4 text-aegis-400" />
                Agent Spending Breakdown
              </h3>
              <span className="text-xs text-gray-500">
                {agents.length} agents tracked
              </span>
            </div>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full text-left">
              <thead>
                <tr className="text-xs font-medium text-gray-500 uppercase tracking-wider border-b border-gray-800">
                  <th className="py-3 px-4 w-10">#</th>
                  <th className="py-3 px-4">Agent</th>
                  <th className="py-3 px-4">Tokens</th>
                  <th className="py-3 px-4">Cost</th>
                  <th className="py-3 px-4">Budget</th>
                  <th className="py-3 px-4">Requests</th>
                  <th className="py-3 px-4">Last Active</th>
                </tr>
              </thead>
              <tbody>
                {agents.map((agent, i) => (
                  <AgentRow
                    key={agent.agent_id}
                    agent={agent}
                    rank={i + 1}
                    budgetCap={10000}
                  />
                ))}
              </tbody>
            </table>
          </div>
        </section>

        {/* Recent Audit Logs */}
        <section className="rounded-xl border border-gray-800 bg-gray-900/50 overflow-hidden">
          <div className="p-5 border-b border-gray-800">
            <h3 className="text-sm font-semibold text-gray-300">Recent Audit Logs</h3>
          </div>
          <div className="overflow-x-auto max-h-80 overflow-y-auto">
            <table className="w-full text-left">
              <thead className="sticky top-0 bg-gray-900">
                <tr className="text-xs font-medium text-gray-500 uppercase tracking-wider border-b border-gray-800">
                  <th className="py-3 px-4">Time</th>
                  <th className="py-3 px-4">Agent</th>
                  <th className="py-3 px-4">Model</th>
                  <th className="py-3 px-4">Prompt</th>
                  <th className="py-3 px-4">Completion</th>
                  <th className="py-3 px-4">Cost</th>
                  <th className="py-3 px-4">Status</th>
                  <th className="py-3 px-4">Request ID</th>
                </tr>
              </thead>
              <tbody>
                {logs.slice(0, 20).map((log, i) => (
                  <tr
                    key={`${log.request_id}-${i}`}
                    className="border-b border-gray-800 hover:bg-gray-800/40 transition-colors"
                  >
                    <td className="py-2 px-4 text-xs text-gray-500">
                      {new Date(log.timestamp).toLocaleTimeString()}
                    </td>
                    <td className="py-2 px-4 text-xs font-mono text-gray-300">
                      {log.agent_id}
                    </td>
                    <td className="py-2 px-4 text-xs text-gray-400">
                      {log.model}
                    </td>
                    <td className="py-2 px-4 text-xs text-gray-400">
                      {log.prompt_tokens.toLocaleString()}
                    </td>
                    <td className="py-2 px-4 text-xs text-gray-400">
                      {log.completion_tokens.toLocaleString()}
                    </td>
                    <td className="py-2 px-4 text-xs font-medium text-gray-300">
                      ${log.total_cost.toFixed(6)}
                    </td>
                    <td className="py-2 px-4">
                      <span
                        className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${
                          log.status_code === 200
                            ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20'
                            : log.status_code === 429
                            ? 'bg-red-500/10 text-red-400 border border-red-500/20'
                            : 'bg-gray-500/10 text-gray-400 border border-gray-500/20'
                        }`}
                      >
                        {log.status_code}
                      </span>
                    </td>
                    <td className="py-2 px-4 text-xs text-gray-600 font-mono">
                      {log.request_id.slice(0, 20)}...
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      </main>
    </div>
  )
}