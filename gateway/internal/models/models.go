package models

// RequestLog represents a single audited request entry.
type RequestLog struct {
    AgentID          string  `json:"agent_id"`
    Model            string  `json:"model"`
    PromptTokens     int     `json:"prompt_tokens"`
    CompletionTokens int     `json:"completion_tokens"`
    TotalCost        float64 `json:"total_cost"`
    RequestID        string  `json:"request_id"`
    StatusCode       int     `json:"status_code"`
    Timestamp        string  `json:"timestamp"`
}

// Stats holds aggregated audit statistics for the dashboard.
type Stats struct {
    TotalTokens   int     `json:"total_tokens"`
    TotalCost     float64 `json:"total_cost"`
    TotalRequests int     `json:"total_requests"`
    AgentCount    int     `json:"agent_count"`
}

// AgentStats holds per-agent aggregated statistics.
type AgentStats struct {
    AgentID             string  `json:"agent_id"`
    TotalTokens         int     `json:"total_tokens"`
    TotalCost           float64 `json:"total_cost"`
    RequestCount        int     `json:"request_count"`
    LastRequestAt       string  `json:"last_request_at"`
    RedactionsTriggered int     `json:"redactions_triggered"`
}
