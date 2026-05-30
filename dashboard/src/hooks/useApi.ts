import { useState, useEffect, useCallback } from 'react'
import { TotalStats, AgentStats, AuditLog } from '../types'

const API_BASE = '/api/v1/stats'
const POLL_INTERVAL = 5000 // 5 seconds

// ──────────────────────────────────────────────
// Mock data for development / demo (used when API unavailable)
// ──────────────────────────────────────────────

const MOCK_TOTAL_STATS: TotalStats = {
  total_tokens: 2_847_391,
  total_cost: 127.43,
  total_requests: 18_294,
  agent_count: 42,
}

const MOCK_AGENT_STATS: AgentStats[] = [
  {
    agent_id: 'sales-research-agent',
    total_tokens: 842_100,
    total_cost: 38.92,
    request_count: 4_812,
    last_request_at: new Date(Date.now() - 45_000).toISOString(),
    redactions_triggered: 23,
  },
  {
    agent_id: 'customer-support-bot',
    total_tokens: 621_450,
    total_cost: 28.14,
    request_count: 3_291,
    last_request_at: new Date(Date.now() - 12_000).toISOString(),
    redactions_triggered: 5,
  },
  {
    agent_id: 'code-review-agent',
    total_tokens: 512_000,
    total_cost: 23.04,
    request_count: 2_100,
    last_request_at: new Date(Date.now() - 3_600_000).toISOString(),
    redactions_triggered: 0,
  },
  {
    agent_id: 'document-analyzer',
    total_tokens: 448_200,
    total_cost: 20.17,
    request_count: 1_890,
    last_request_at: new Date(Date.now() - 120_000).toISOString(),
    redactions_triggered: 41,
  },
  {
    agent_id: 'market-intelligence-v2',
    total_tokens: 423_641,
    total_cost: 17.16,
    request_count: 2_201,
    last_request_at: new Date(Date.now() - 8_000).toISOString(),
    redactions_triggered: 7,
  },
]

const MOCK_RECENT_LOGS: AuditLog[] = Array.from({ length: 20 }, (_, i) => ({
  agent_id: MOCK_AGENT_STATS[i % MOCK_AGENT_STATS.length].agent_id,
  model: ['gpt-4', 'gpt-3.5-turbo', 'gpt-4-turbo-preview'][i % 3],
  prompt_tokens: Math.floor(Math.random() * 2000) + 100,
  completion_tokens: Math.floor(Math.random() * 500) + 50,
  total_cost: parseFloat((Math.random() * 0.15).toFixed(6)),
  request_id: `aegis-${Date.now() - i * 15_000}`,
  status_code: i === 3 ? 429 : 200,
  timestamp: new Date(Date.now() - i * 15_000).toISOString(),
}))

// ──────────────────────────────────────────────
// Fetch helpers
// ──────────────────────────────────────────────

async function fetchJson<T>(url: string, fallback: T): Promise<T> {
  try {
    const resp = await fetch(url)
    if (!resp.ok) return fallback
    return resp.json() as Promise<T>
  } catch {
    return fallback
  }
}

// ──────────────────────────────────────────────
// Hooks
// ──────────────────────────────────────────────

export function useTotalStats() {
  const [data, setData] = useState<TotalStats>(MOCK_TOTAL_STATS)
  const [loading, setLoading] = useState(true)

  const refresh = useCallback(async () => {
    const result = await fetchJson<TotalStats>(`${API_BASE}/totals`, MOCK_TOTAL_STATS)
    setData(result)
    setLoading(false)
  }, [])

  useEffect(() => {
    refresh()
    const interval = setInterval(refresh, POLL_INTERVAL)
    return () => clearInterval(interval)
  }, [refresh])

  return { data, loading }
}

export function useAgentStats() {
  const [data, setData] = useState<AgentStats[]>(MOCK_AGENT_STATS)
  const [loading, setLoading] = useState(true)

  const refresh = useCallback(async () => {
    const result = await fetchJson<AgentStats[]>(`${API_BASE}/agents`, MOCK_AGENT_STATS)
    setData(result)
    setLoading(false)
  }, [])

  useEffect(() => {
    refresh()
    const interval = setInterval(refresh, POLL_INTERVAL)
    return () => clearInterval(interval)
  }, [refresh])

  return { data, loading }
}

export function useRecentLogs(limit = 50) {
  const [data, setData] = useState<AuditLog[]>(MOCK_RECENT_LOGS)
  const [loading, setLoading] = useState(true)

  const refresh = useCallback(async () => {
    const result = await fetchJson<AuditLog[]>(
      `${API_BASE}/logs?limit=${limit}`,
      MOCK_RECENT_LOGS,
    )
    setData(result)
    setLoading(false)
  }, [limit])

  useEffect(() => {
    refresh()
    const interval = setInterval(refresh, POLL_INTERVAL)
    return () => clearInterval(interval)
  }, [refresh])

  return { data, loading }
}
