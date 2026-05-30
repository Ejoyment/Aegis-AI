// Core data types for the AegisAI FinOps Dashboard

export interface TotalStats {
  total_tokens: number
  total_cost: number
  total_requests: number
  agent_count: number
}

export interface AgentStats {
  agent_id: string
  total_tokens: number
  total_cost: number
  request_count: number
  last_request_at: string
  redactions_triggered: number
}

export interface AuditLog {
  agent_id: string
  model: string
  prompt_tokens: number
  completion_tokens: number
  total_cost: number
  request_id: string
  status_code: number
  timestamp: string
}

export interface TimeSeriesPoint {
  time: string
  tokens: number
  cost: number
  requests: number
}
