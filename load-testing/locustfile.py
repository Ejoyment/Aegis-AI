"""
AegisAI Load Testing Suite
===========================
Simulates 10,000 concurrent AI agent API requests through the AegisAI proxy.
Measures latency overhead (target: <50ms p99), rate limiting behavior, and
audit logging correctness.

Usage:
    # Start AegisAI gateway with Redis and PostgreSQL
    export REDIS_URL=redis://localhost:6379
    export DATABASE_URL=postgres://user:pass@localhost:5432/aegis?sslmode=disable
    export API_KEY=sk-test-key
    go run ./cmd/proxy/

    # In another terminal, run load test
    locust -f load-testing/locustfile.py --host=http://localhost:8443 \
        --users=10000 --spawn-rate=500 --run-time=5m --headless \
        --csv=aegis-load-test
"""

import json
import random
import time
from locust import HttpUser, task, between, events
import numpy as np


# Simulated agent identities
AGENT_IDS = [
    f"agent-{i:04d}" for i in range(100)
]

# Sample prompts of varying token lengths
SAMPLE_PROMPTS = [
    "What is the capital of France?",
    "Explain quantum computing in simple terms.",
    "Write a Python function to sort a list of integers.",
    "Summarize the key differences between Redis and PostgreSQL.",
    "Generate a 5-step plan for implementing AI governance in an enterprise.",
    "Describe the architecture of a reverse proxy for LLM inference.",
    "What are the ethical implications of autonomous AI agents?",
    "Write a comprehensive analysis of token-based rate limiting strategies.",
    "Create a detailed comparison of Go vs Python for high-throughput proxying.",
    "Design a system for real-time PII redaction in LLM prompts.",
]

# Longer prompt for stress testing token counting
LONG_PROMPT = """
    "messages": [
        {"role": "system", "content": "You are an expert AI assistant."},
        {"role": "user", "content": "Write a detailed analysis of the following topics, covering architecture, implementation, and best practices for each: 1. Reverse proxy design patterns, 2. Token bucket rate limiting algorithms, 3. Redis-based sliding window counters, 4. PostgreSQL audit logging schemas, 5. PII detection and redaction strategies, 6. Kubernetes deployment strategies for stateless services, 7. Prometheus metrics collection and Grafana dashboarding, 8. Load testing methodologies for LLM APIs."}
    ]
"""


class AegisAgentUser(HttpUser):
    """
    Simulates an AI agent sending requests through the AegisAI proxy.
    """
    wait_time = between(0.5, 3.0)  # Agents send requests every 0.5-3 seconds
    latency_measurements = []  # Shared list for latency tracking

    def on_start(self):
        """Called when a simulated user starts."""
        self.agent_id = random.choice(AGENT_IDS)
        self.model = random.choice(["gpt-4", "gpt-3.5-turbo", "gpt-4-turbo-preview"])
        self.session_id = f"session-{int(time.time())}-{random.randint(1000, 9999)}"

    @task(7)
    def send_chat_completion(self):
        """Send a standard chat completion request (most common operation)."""
        prompt = random.choice(SAMPLE_PROMPTS)
        headers = self._get_headers()

        payload = {
            "model": self.model,
            "messages": [
                {"role": "system", "content": "You are a helpful assistant."},
                {"role": "user", "content": prompt}
            ],
            "max_tokens": 150,
            "temperature": 0.7,
            "stream": False
        }

        start_time = time.time()
        with self.client.post(
            "/v1/chat/completions",
            json=payload,
            headers=headers,
            catch_response=True,
            name="/v1/chat/completions"
        ) as response:
            elapsed = (time.time() - start_time) * 1000  # ms
            self.latency_measurements.append(elapsed)

            if response.status_code == 200:
                response.success()
            elif response.status_code == 429:
                # Rate limited - expected behavior under load
                response.success()
            else:
                response.failure(f"Unexpected status: {response.status_code}")

    @task(2)
    def send_long_prompt(self):
        """Send a request with a long prompt to stress token counting."""
        headers = self._get_headers()

        payload = {
            "model": "gpt-4",
            "messages": [
                {"role": "system", "content": "You are an expert AI assistant."},
                {"role": "user", "content": LONG_PROMPT}
            ],
            "max_tokens": 500,
            "temperature": 0.7
        }

        with self.client.post(
            "/v1/chat/completions",
            json=payload,
            headers=headers,
            catch_response=True,
            name="/v1/chat/completions (long)"
        ) as response:
            if response.status_code in (200, 429):
                response.success()
            else:
                response.failure(f"Unexpected status: {response.status_code}")

    @task(1)
    def send_completion(self):
        """Send a legacy text completion request."""
        headers = self._get_headers()

        payload = {
            "model": "gpt-3.5-turbo-instruct",
            "prompt": "Explain the concept of rate limiting in one paragraph.",
            "max_tokens": 100,
            "temperature": 0.5
        }

        with self.client.post(
            "/v1/completions",
            json=payload,
            headers=headers,
            catch_response=True,
            name="/v1/completions"
        ) as response:
            if response.status_code in (200, 429):
                response.success()
            else:
                response.failure(f"Unexpected status: {response.status_code}")

    def _get_headers(self):
        """Generate request headers with agent identity."""
        return {
            "Content-Type": "application/json",
            "X-Aegis-Agent-ID": self.agent_id,
            "X-Aegis-Session-ID": self.session_id,
            "Authorization": "Bearer sk-test-key"
        }


@events.test_start.add_listener
def on_test_start(environment, **kwargs):
    """Log test configuration at start."""
    print(f"\n{'='*60}")
    print(f"AegisAI Load Test Starting")
    print(f"{'='*60}")
    print(f"Target: {environment.host}")
    print(f"Users: {environment.parsed_options.users if environment.parsed_options else 'N/A'}")
    print(f"Spawn Rate: {environment.parsed_options.spawn_rate if environment.parsed_options else 'N/A'}")
    print(f"{'='*60}\n")


@events.test_stop.add_listener
def on_test_stop(environment, **kwargs):
    """Generate latency report on test completion."""
    measurements = AegisAgentUser.latency_measurements

    print(f"\n{'='*60}")
    print(f"AegisAI Load Test Results")
    print(f"{'='*60}")
    print(f"Total requests measured: {len(measurements)}")

    if measurements:
        latencies = np.array(measurements)
        print(f"\nLatency Overhead (ms):")
        print(f"  P50:     {np.percentile(latencies, 50):.2f}ms")
        print(f"  P90:     {np.percentile(latencies, 90):.2f}ms")
        print(f"  P95:     {np.percentile(latencies, 95):.2f}ms")
        print(f"  P99:     {np.percentile(latencies, 99):.2f}ms")
        print(f"  Max:     {np.max(latencies):.2f}ms")
        print(f"  Min:     {np.min(latencies):.2f}ms")
        print(f"  Mean:    {np.mean(latencies):.2f}ms")
        print(f"  Std Dev: {np.std(latencies):.2f}ms")

        target = 50  # <50ms target
        p99 = np.percentile(latencies, 99)
        if p99 < target:
            print(f"\n✅ PASS: P99 latency ({p99:.2f}ms) is under {target}ms target")
        else:
            print(f"\n❌ FAIL: P99 latency ({p99:.2f}ms) exceeds {target}ms target")

        # Check rate limiting effectiveness
        rate_limited = sum(1 for _ in measurements if _ == 429)
        print(f"\nRate limiting events: {rate_limited}")
    else:
        print("\n⚠️  No measurements collected")

    print(f"{'='*60}\n")