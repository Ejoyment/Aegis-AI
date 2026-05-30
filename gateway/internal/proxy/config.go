package proxy

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all configuration for the AegisAI proxy gateway.
type Config struct {
	// Port is the listening port for the proxy server.
	Port int

	// UpstreamURL is the base URL of the LLM provider (e.g., https://api.openai.com).
	UpstreamURL string

	// RedisURL is the connection string for Redis (rate limiting & budgeting).
	RedisURL string

	// DatabaseURL is the PostgreSQL connection string (audit logging).
	DatabaseURL string

	// SLMRedactorURL is the URL of the local SLM PII redaction service.
	SLMRedactorURL string

	// MaxTokensPerMinute is the default token limit per agent per minute.
	MaxTokensPerMinute int

	// MaxCostPerDay is the default daily cost cap per agent (in USD cents).
	MaxCostPerDay int

	// APIKey is the upstream API key (e.g., OpenAI API key).
	APIKey string
}

// LoadConfig reads configuration from environment variables with sensible defaults.
func LoadConfig() (*Config, error) {
	port, err := getEnvInt("PORT", 8443)
	if err != nil {
		return nil, fmt.Errorf("invalid PORT: %w", err)
	}

	upstreamURL := os.Getenv("UPSTREAM_URL")
	if upstreamURL == "" {
		upstreamURL = "https://api.openai.com"
	}

	maxTokens, err := getEnvInt("MAX_TOKENS_PER_MIN", 100000)
	if err != nil {
		return nil, fmt.Errorf("invalid MAX_TOKENS_PER_MIN: %w", err)
	}

	maxCost, err := getEnvInt("MAX_COST_PER_DAY", 10000) // $100.00 in cents
	if err != nil {
		return nil, fmt.Errorf("invalid MAX_COST_PER_DAY: %w", err)
	}

	return &Config{
		Port:               port,
		UpstreamURL:        upstreamURL,
		RedisURL:           os.Getenv("REDIS_URL"),
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		SLMRedactorURL:     os.Getenv("SLM_REDACTOR_URL"),
		MaxTokensPerMinute: maxTokens,
		MaxCostPerDay:      maxCost,
		APIKey:             os.Getenv("API_KEY"),
	}, nil
}

func getEnvInt(key string, defaultVal int) (int, error) {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal, nil
	}
	i, err := strconv.Atoi(val)
	if err != nil {
		return 0, err
	}
	return i, nil
}
