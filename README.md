# AegisAI — Enterprise Agentic AI Governance Gateway

![Version](https://img.shields.io/badge/version-1.0.0-blue)
![Go](https://img.shields.io/badge/Go-1.22+-00ADD8)
![License](https://img.shields.io/badge/license-MIT-green)

**AegisAI is a self-hosted, ultra-low latency API gateway for enterprise AI agent governance and FinOps.**

Stop burning API budgets on runaway AI agents. AegisAI sits between your agents and LLM providers, intercepting every request to enforce token budgets, rate limits, and PII redaction — entirely on your infrastructure.

## Why AegisAI?

> *"Your autonomous agents are a financial black box."*

- **💰 FinOps Control** — Real-time token and cost tracking per agent. Hard budget caps prevent cost overruns.
- **🛡️ Security** — On-premise PII redaction ensures sensitive data never leaves your network.
- **⚡ Performance** — Go-based proxy with <50ms latency overhead at 10,000 concurrent requests.
- **📊 Visibility** — CFO-grade FinOps dashboard showing exactly what every agent is spending.

## Architecture

```
AI Agent → AegisAI Gateway → Rate Limit Check → PII Redact → LLM Provider
                  ↓
           PostgreSQL Audit Log ─→ FinOps Dashboard
```

## Quick Start

```bash
# Deploy in under 5 minutes
export API_KEY="sk-your-openai-key"
docker compose -f deploy/docker-compose.yml up -d

# Route agents through AegisAI
curl -X POST http://localhost:8443/v1/chat/completions \
  -H "X-Aegis-Agent-ID: my-agent" \
  -d '{"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}'

# Open the dashboard
open http://localhost:5173
```

## Project Structure

```
├── gateway/           # Go reverse proxy (core service)
│   ├── cmd/proxy/     # Entry point
│   └── internal/      # Proxy, token, ratelimit, audit, redact
├── slm-redactor/      # On-premise PII redaction (Python/FastAPI)
├── dashboard/         # React FinOps dashboard (Vite + Tailwind)
├── load-testing/      # Locust load tests & penetration tests
├── deploy/            # Helm charts, docker-compose, SQL migrations
└── docs/              # Architecture, quickstart, API reference
```

## 12-Week MVP Roadmap

| Phase | Weeks | Deliverables |
|-------|-------|-------------|
| **Core Proxy** | 1-4 | Go reverse proxy, token counting (tiktoken), Redis rate limiting, PostgreSQL audit |
| **Security** | 5-8 | Load testing (10k concurrent), PII redaction (Presidio), pen testing, FinOps dashboard |
| **Launch** | 9-12 | Helm charts, documentation, docker-compose, staging infra, beta test, go-live |

Current status: **12-week MVP complete** ✅

## Features

- **Token-based rate limiting**: Sliding window counters in Redis per agent/minute
- **Daily cost budgets**: Hard caps per agent (configurable, stored in Redis)
- **PII redaction**: Detects emails, SSNs, API keys, credit cards, crypto addresses using Microsoft Presidio
- **Immutable audit logs**: Every request logged to PostgreSQL with token usage and cost
- **FinOps dashboard**: Real-time spend visualization, agent breakdowns, budget alerts
- **Prometheus metrics**: `/metrics` endpoint with request counts, latency, and rate limit stats
- **Helm deployment**: Ready for air-gapped AWS EKS with auto-scaling

## Pricing Tiers

| Tier | Price | Features |
|------|-------|----------|
| **Growth** | $45,000/yr | 50 agents, 500M tokens, standard VPC, basic audit |
| **Enterprise** | $120,000+/yr | Unlimited agents/tokens, custom SLM models, 24/7 support |

## License

Confidential — AegisAI Founding Team. Internal use only.