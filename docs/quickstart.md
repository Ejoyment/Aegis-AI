# AegisAI Quickstart Guide

Deploy the AegisAI gateway in under 5 minutes using Docker Compose.

## Prerequisites

- Docker & Docker Compose v2
- An OpenAI API key (or compatible LLM provider)

## Quick Deploy

```bash
# 1. Clone and enter the repo
git clone https://github.com/Ejoyment/Aegis-AI.git
cd Aegis-AI

# 2. Set your API key
export API_KEY="sk-your-openai-key-here"

# 3. Launch all services
docker compose -f deploy/docker-compose.yml up -d

# 4. Verify everything is running
docker compose ps
```

## Verify

```bash
# Health check
curl http://localhost:8443/health

# Send a test request
curl -X POST http://localhost:8443/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "X-Aegis-Agent-ID: my-test-agent" \
  -d '{
    "model": "gpt-3.5-turbo",
    "messages": [{"role": "user", "content": "Hello!"}],
    "max_tokens": 50
  }'

# Check the dashboard
open http://localhost:5173
```

## Services

| Service       | URL                          | Description              |
|---------------|------------------------------|--------------------------|
| Proxy         | http://localhost:8443        | LLM request gateway      |
| Dashboard     | http://localhost:5173        | FinOps monitoring UI     |
| Redactor API  | http://localhost:8000/redact | PII detection endpoint   |
| PostgreSQL    | localhost:5432               | Audit log database       |
| Redis         | localhost:6379               | Rate limit state         |

## Run Load Tests

```bash
# Simulate 10,000 concurrent agents
locust -f load-testing/locustfile.py --host=http://localhost:8443 \
  --users=10000 --spawn-rate=500 --run-time=5m --headless

# Run penetration tests
python load-testing/penetration.py
```

## Configuration

Set environment variables before starting:

| Variable              | Default                    | Description                |
|-----------------------|----------------------------|----------------------------|
| `PORT`                | `8443`                     | Proxy listen port          |
| `UPSTREAM_URL`        | `https://api.openai.com`   | LLM provider base URL      |
| `REDIS_URL`           | `redis://localhost:6379`   | Redis connection string    |
| `DATABASE_URL`        | (optional)                 | PostgreSQL connection      |
| `SLM_REDACTOR_URL`    | (optional)                 | PII redactor service URL   |
| `MAX_TOKENS_PER_MIN`  | `100000`                   | Per-agent token limit/min  |
| `MAX_COST_PER_DAY`    | `10000` ($100)             | Per-agent daily cost cap   |
| `API_KEY`             | (required)                 | Upstream LLM API key       |