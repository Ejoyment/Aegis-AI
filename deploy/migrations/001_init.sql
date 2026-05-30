-- AegisAI Audit Log — PostgreSQL Schema
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
CREATE INDEX IF NOT EXISTS idx_audit_agent_id ON audit_logs(agent_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_created_at ON audit_logs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_request_id ON audit_logs(request_id);