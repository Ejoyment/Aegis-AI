# AegisAI

![Go](https://img.shields.io/badge/Go-1.22+-00ADD8)
![License](https://img.shields.io/badge/license-MIT-green)

A self-hosted API gateway that sits between AI agents and LLM providers, built to track token spend, enforce rate limits, and redact PII before requests leave your infrastructure.

I started this project to understand how production AI infrastructure handles cost control and governance for autonomous agents — a problem that comes up as more teams deploy LLM-based agents that make their own API calls.

## How it works

AI Agent → AegisAI Gateway → Rate Limit Check → PII Redact → LLM Provider
↓
PostgreSQL Audit Log ─→ Dashboard


## Quick Start

```bash
export API_KEY="sk-your-openai-key"
docker compose -f deploy/docker-compose.yml up -d

curl -X POST http://localhost:8443/v1/chat/completions \
  -H "X-Aegis-Agent-ID: my-agent" \
  -d '{"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}'

open http://localhost:5173  # dashboard
```

## Project Structure

├── gateway/ # Go reverse proxy (core service)
│ ├── cmd/proxy/ # Entry point
│ └── internal/ # Proxy, token, ratelimit, audit, redact
├── slm-redactor/ # On-premise PII redaction (Python/FastAPI + Presidio)
├── dashboard/ # React dashboard (Vite + Tailwind)
├── load-testing/ # Locust load tests
├── deploy/ # docker-compose, SQL migrations
└── docs/ # Architecture, API reference


## What's working

- **Reverse proxy** — forwards `/v1/chat/completions` and `/v1/completions` to a configured upstream LLM provider
- **PII redaction** — real Presidio-based analyzer/anonymizer via FastAPI (`slm-redactor/`), detects and redacts PII (emails, SSNs, API keys, credit cards) when the service is running
- **Audit logging** — every proxied request is written to PostgreSQL (`gateway/internal/audit/`)
- **Rate limiting** — Redis-backed token-window and daily-cost limit checks (`gateway/internal/ratelimit/`)

## In progress

- **Dashboard stats** — the UI is built and fetches from the stats API, but the backend endpoints currently return placeholder data instead of aggregated results from the audit log. Wiring real aggregation is the next milestone.

## Known limitations

- Basic reverse proxy behavior, not a hardened SDK-level integration
- Redaction and rate-limiting require their respective services (`slm-redactor`, Redis) to be configured and running — they no-op or fail to start otherwise
- Not yet load-tested at the scale described in early design docs; `load-testing/` contains scripts for this but results aren't published

## Roadmap

- Wire dashboard to real aggregated audit data
- Prometheus metrics endpoint
- Helm chart for Kubernetes deployment

## License

MIT