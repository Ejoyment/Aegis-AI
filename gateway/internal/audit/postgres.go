package audit

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Ejoyment/Aegis-AI/gateway/internal/proxy"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresLogger implements the AuditLogger interface using PostgreSQL.
type PostgresLogger struct {
	pool *pgxpool.Pool
	ctx  context.Context
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

// NewPostgresLogger creates a new PostgreSQL audit logger.
// If databaseURL is empty, it returns a no-op logger.
func NewPostgresLogger(databaseURL string) (*PostgresLogger, error) {
	if databaseURL == "" {
		log.Println("Database URL not configured — audit logging disabled")
		return &PostgresLogger{pool: nil}, nil
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	// Run migrations
	if err := runMigrations(ctx, pool); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	log.Printf("Connected to PostgreSQL audit database")
	return &PostgresLogger{
		pool: pool,
		ctx:  ctx,
	}, nil
}

// Log persists a request audit log entry to PostgreSQL.
func (l *PostgresLogger) Log(entry *proxy.RequestLog) error {
	if l.pool == nil {
		// Audit logging disabled — just log to stdout
		log.Printf("[AUDIT] Agent=%s Model=%s Prompt=%d Completion=%d Cost=$%.6f Status=%d",
			entry.AgentID, entry.Model, entry.PromptTokens, entry.CompletionTokens,
			entry.TotalCost, entry.StatusCode)
		return nil
	}

	query := `
		INSERT INTO audit_logs (
			agent_id, model, prompt_tokens, completion_tokens,
			total_cost, request_id, status_code, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := l.pool.Exec(l.ctx, query,
		entry.AgentID,
		entry.Model,
		entry.PromptTokens,
		entry.CompletionTokens,
		entry.TotalCost,
		entry.RequestID,
		entry.StatusCode,
		entry.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("failed to insert audit log: %w", err)
	}

	return nil
}

// GetTotalStats returns aggregate statistics across all agents.
func (l *PostgresLogger) GetTotalStats() (*Stats, error) {
	if l.pool == nil {
		return &Stats{}, nil
	}

	query := `
		SELECT
			COALESCE(SUM(prompt_tokens + completion_tokens), 0) as total_tokens,
			COALESCE(SUM(total_cost), 0) as total_cost,
			COUNT(*) as total_requests,
			COUNT(DISTINCT agent_id) as agent_count
		FROM audit_logs
		WHERE created_at >= NOW() - INTERVAL '30 days'
	`

	var stats Stats
	err := l.pool.QueryRow(l.ctx, query).Scan(
		&stats.TotalTokens,
		&stats.TotalCost,
		&stats.TotalRequests,
		&stats.AgentCount,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query total stats: %w", err)
	}

	return &stats, nil
}

// GetAgentStats returns aggregated statistics per agent.
func (l *PostgresLogger) GetAgentStats() ([]AgentStats, error) {
	if l.pool == nil {
		return []AgentStats{}, nil
	}

	query := `
		SELECT
			agent_id,
			COALESCE(SUM(prompt_tokens + completion_tokens), 0) as total_tokens,
			COALESCE(SUM(total_cost), 0) as total_cost,
			COUNT(*) as request_count,
			MAX(created_at) as last_request_at
		FROM audit_logs
		WHERE created_at >= NOW() - INTERVAL '30 days'
		GROUP BY agent_id
		ORDER BY total_cost DESC
	`

	rows, err := l.pool.Query(l.ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query agent stats: %w", err)
	}
	defer rows.Close()

	var stats []AgentStats
	for rows.Next() {
		var s AgentStats
		var lastRequestAt time.Time
		if err := rows.Scan(
			&s.AgentID,
			&s.TotalTokens,
			&s.TotalCost,
			&s.RequestCount,
			&lastRequestAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan agent stats row: %w", err)
		}
		s.LastRequestAt = lastRequestAt.Format(time.RFC3339)
		stats = append(stats, s)
	}

	return stats, nil
}

// GetRecentLogs returns the most recent audit log entries.
func (l *PostgresLogger) GetRecentLogs(limit int) ([]proxy.RequestLog, error) {
	if l.pool == nil {
		return []proxy.RequestLog{}, nil
	}

	if limit <= 0 {
		limit = 100
	}

	query := `
		SELECT agent_id, model, prompt_tokens, completion_tokens,
			   total_cost, request_id, status_code, created_at
		FROM audit_logs
		ORDER BY created_at DESC
		LIMIT $1
	`

	rows, err := l.pool.Query(l.ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent logs: %w", err)
	}
	defer rows.Close()

	var logs []proxy.RequestLog
	for rows.Next() {
		var entry proxy.RequestLog
		var createdAt time.Time
		if err := rows.Scan(
			&entry.AgentID,
			&entry.Model,
			&entry.PromptTokens,
			&entry.CompletionTokens,
			&entry.TotalCost,
			&entry.RequestID,
			&entry.StatusCode,
			&createdAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan log row: %w", err)
		}
		entry.Timestamp = createdAt.Format(time.RFC3339)
		logs = append(logs, entry)
	}

	return logs, nil
}

// GetAgentLogs returns audit logs for a specific agent.
func (l *PostgresLogger) GetAgentLogs(agentID string, limit int) ([]proxy.RequestLog, error) {
	if l.pool == nil {
		return []proxy.RequestLog{}, nil
	}

	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT agent_id, model, prompt_tokens, completion_tokens,
			   total_cost, request_id, status_code, created_at
		FROM audit_logs
		WHERE agent_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := l.pool.Query(l.ctx, query, agentID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query agent logs: %w", err)
	}
	defer rows.Close()

	var logs []proxy.RequestLog
	for rows.Next() {
		var entry proxy.RequestLog
		var createdAt time.Time
		if err := rows.Scan(
			&entry.AgentID,
			&entry.Model,
			&entry.PromptTokens,
			&entry.CompletionTokens,
			&entry.TotalCost,
			&entry.RequestID,
			&entry.StatusCode,
			&createdAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan log row: %w", err)
		}
		entry.Timestamp = createdAt.Format(time.RFC3339)
		logs = append(logs, entry)
	}

	return logs, nil
}

// Close closes the PostgreSQL connection pool.
func (l *PostgresLogger) Close() error {
	if l.pool != nil {
		l.pool.Close()
	}
	return nil
}

// runMigrations ensures the database schema is up to date.
func runMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	// Create the audit_logs table if it doesn't exist
	migration := `
		CREATE TABLE IF NOT EXISTS audit_logs (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			agent_id VARCHAR(128) NOT NULL,
			model VARCHAR(64) NOT NULL DEFAULT '',
			prompt_tokens INT NOT NULL DEFAULT 0,
			completion_tokens INT NOT NULL DEFAULT 0,
			total_cost NUMERIC(12,6) NOT NULL DEFAULT 0,
			request_id VARCHAR(64) NOT NULL,
			status_code INT NOT NULL DEFAULT 200,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE INDEX IF NOT EXISTS idx_audit_agent_id
			ON audit_logs(agent_id, created_at DESC);

		CREATE INDEX IF NOT EXISTS idx_audit_created_at
			ON audit_logs(created_at DESC);

		CREATE INDEX IF NOT EXISTS idx_audit_request_id
			ON audit_logs(request_id);
	`

	_, err := pool.Exec(ctx, migration)
	if err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	log.Println("Database migrations completed successfully")
	return nil
}

// Ensure PostgresLogger implements proxy.AuditLogger.
var _ proxy.AuditLogger = (*PostgresLogger)(nil)
