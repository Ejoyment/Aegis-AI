# AegisAI Architecture

## Overview

AegisAI is a self-hosted, ultra-low latency API gateway designed exclusively for enterprise Agentic AI governance and FinOps. It sits between AI agents and LLM providers (OpenAI, Anthropic, etc.), intercepting every request to enforce budgets, rate limits, and PII redaction.

## Core Components

```
┌─────────────────────────────────────────────────────────┐
│                     AI Agents                            │
│  (sales-research-agent, support-bot, code-reviewer...)  │
└────────────────────────┬────────────────────────────────┘
                         │ X-Aegis-Agent-ID header
                         ▼
┌─────────────────────────────────────────────────────────┐
│                AegisAI Gateway (Go)                      │
│                                                          │
│  ┌──────────┐  ┌───────────┐  ┌──────────────────────┐  │
│  │  Proxy   │  │   Token   │  │  Rate Limiter        │  │
│  │  Engine  │──│  Counter  │──│  (Redis Sliding Win) │  │
│  └──────────┘  └───────────┘  └──────────┬───────────┘  │
│                                           │              │
│  ┌──────────────────────┐  ┌──────────────┘              │
│  │  PII Redactor Client │──│ (calls SLM service)         │
│  └──────────────────────┘  └─────────────────────────────│
│                                                          │
│  ┌──────────────────────────────────────────────────┐    │
│  │  Audit Logger (PostgreSQL)                       │    │
│  └──────────────────────────────────────────────────┘    │
└────────────────────────┬────────────────────────────────┘
                         │ Forwarded with API key
                         ▼
┌─────────────────────────────────────────────────────────┐
│              LLM Provider (OpenAI/Anthropic)             │
└─────────────────────────────────────────────────────────┘
```

## Data Flow

1. **Agent sends request** → adds `X-Aegis-Agent-ID` header
2. **Proxy intercepts** → reads request body (model, messages)
3. **Token counter** → estimates prompt tokens using tiktoken
4. **Rate limiter** → checks Redis sliding window for per-agent token/minute and daily cost budget
5. **PII Redactor** → calls on-premise SLM service to detect and mask PII
6. **Forwarding** → sends modified request to upstream LLM with API key
7. **Response capture** → extracts usage data from response
8. **Audit logging** → inserts immutable record to PostgreSQL
9. **Response returned** → transparently returns to agent

## Key Design Decisions

- **Go** for the proxy: high throughput, low latency, excellent concurrency
- **Redis** sorted sets for sliding window rate limiting: O(log N) operations, sub-millisecond latency
- **PostgreSQL** for audit logs: ACID compliance, rich querying for dashboard
- **Python/Presidio** for PII redaction: mature NLP library, runs entirely on-premise
- **React/Vite** for dashboard: fast development, real-time updates via polling