package ratelimit

import (
	"context"
	"fmt"
	"log"
	"math"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// Key prefixes for Redis storage
	keyPrefixTokens = "aegis:tokens:"
	keyPrefixBudget = "aegis:budget:"
	keyPrefixConfig = "aegis:config:"

	// Default limits
	defaultTokensPerMinute = 100000
	defaultCostPerDay      = 10000 // $100.00 in cents

	// Window sizes
	minuteWindow = 60    // seconds
	dayWindow    = 86400 // seconds
)

// RedisRateLimiter implements token-based rate limiting using Redis.
// Uses a sliding window algorithm for per-minute token counting
// and daily cost budgets.
type RedisRateLimiter struct {
	client *redis.Client
	ctx    context.Context
}

// Config holds per-agent rate limiting configuration.
type Config struct {
	TokensPerMinute int // Maximum tokens allowed per minute
	CostPerDay      int // Maximum cost in USD cents per day
}

// NewRedisRateLimiter creates a new Redis-backed rate limiter.
// If redisURL is empty, it returns a no-op limiter.
func NewRedisRateLimiter(redisURL string) (*RedisRateLimiter, error) {
	if redisURL == "" {
		log.Println("Redis URL not configured — rate limiting disabled")
		return &RedisRateLimiter{client: nil}, nil
	}

	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid Redis URL: %w", err)
	}

	client := redis.NewClient(opts)

	// Verify connection
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	log.Printf("Connected to Redis at %s", opts.Addr)
	return &RedisRateLimiter{
		client: client,
		ctx:    ctx,
	}, nil
}

// Allow checks whether an agent is allowed to send a request consuming `tokens` tokens.
// Returns: allowed (bool), remaining (int), error.
// The remaining count represents tokens remaining in the current minute window.
func (r *RedisRateLimiter) Allow(agentID string, tokens int) (bool, int, error) {
	if r.client == nil {
		// No Redis configured — allow everything
		return true, math.MaxInt32, nil
	}

	now := time.Now().Unix()

	// Check minute-level token rate limit
	allowed, remaining, err := r.checkMinuteLimit(agentID, tokens, now)
	if err != nil {
		return false, 0, fmt.Errorf("minute limit check failed: %w", err)
	}
	if !allowed {
		return false, 0, nil
	}

	// Check daily cost budget
	allowed, err = r.checkDailyBudget(agentID, tokens, now)
	if err != nil {
		return false, 0, fmt.Errorf("daily budget check failed: %w", err)
	}
	if !allowed {
		return false, 0, nil
	}

	return true, remaining, nil
}

// checkMinuteLimit implements a sliding window counter for token usage per minute.
func (r *RedisRateLimiter) checkMinuteLimit(agentID string, tokens int, now int64) (bool, int, error) {
	key := keyPrefixTokens + agentID
	limit := r.getAgentConfig(agentID).TokensPerMinute

	// Use a Redis Lua script for atomic sliding window
	script := `
		local key = KEYS[1]
		local now = tonumber(ARGV[1])
		local window = tonumber(ARGV[2])
		local limit = tonumber(ARGV[3])
		local tokens = tonumber(ARGV[4])

		-- Remove entries outside the window
		redis.call('ZREMRANGEBYSCORE', key, 0, now - window)

		-- Sum tokens in current window
		local total = 0
		local entries = redis.call('ZRANGE', key, 0, -1, 'WITHSCORES')
		for i = 1, #entries, 2 do
			local entry = redis.call('ZSCORE', key, entries[i])
			if entry then
				total = total + tonumber(entries[i])
			end
		end

		-- Check if adding tokens would exceed limit
		if total + tokens > limit then
			local remaining = limit - total
			if remaining < 0 then remaining = 0 end
			return {0, remaining}
		end

		-- Add current request tokens
		local member = tostring(tokens) .. ":" .. tostring(now)
		redis.call('ZADD', key, now, member)
		redis.call('EXPIRE', key, window + 10)

		local remaining = limit - (total + tokens)
		return {1, remaining}
	`

	result, err := r.client.Eval(r.ctx, script, []string{key}, now, minuteWindow, limit, tokens).Result()
	if err != nil {
		return false, 0, err
	}

	vals, ok := result.([]interface{})
	if !ok || len(vals) != 2 {
		return false, 0, fmt.Errorf("unexpected Redis response format")
	}

	allowed := vals[0].(int64) == 1
	remaining := int(vals[1].(int64))

	return allowed, remaining, nil
}

// checkDailyBudget verifies the agent hasn't exceeded its daily cost cap.
func (r *RedisRateLimiter) checkDailyBudget(agentID string, tokens int, now int64) (bool, error) {
	key := keyPrefixBudget + agentID
	cfg := r.getAgentConfig(agentID)

	// Simplified cost estimation: assume worst-case GPT-4 pricing
	// $0.03 per 1K prompt + $0.06 per 1K completion ≈ $0.09 per 1K tokens
	// Convert to cents: 0.009 cents per token
	costCents := int(float64(tokens) * 0.009)

	script := `
		local key = KEYS[1]
		local now = tonumber(ARGV[1])
		local window = tonumber(ARGV[2])
		local budget = tonumber(ARGV[3])
		local cost = tonumber(ARGV[4])

		-- Remove entries outside the window
		redis.call('ZREMRANGEBYSCORE', key, 0, now - window)

		-- Sum cost in current window
		local total = 0
		local entries = redis.call('ZRANGE', key, 0, -1, 'WITHSCORES')
		for i = 1, #entries, 2 do
			total = total + tonumber(entries[i])
		end

		-- Check budget
		if total + cost > budget then
			return 0
		end

		-- Add current cost
		local member = tostring(cost) .. ":" .. tostring(now)
		redis.call('ZADD', key, now, member)
		redis.call('EXPIRE', key, window + 10)

		return 1
	`

	result, err := r.client.Eval(r.ctx, script, []string{key}, now, dayWindow, cfg.CostPerDay, costCents).Result()
	if err != nil {
		return false, err
	}

	allowed := result.(int64) == 1
	return allowed, nil
}

// getAgentConfig retrieves per-agent rate limiting configuration from Redis.
// Falls back to defaults if no config is stored.
func (r *RedisRateLimiter) getAgentConfig(agentID string) Config {
	if r.client == nil {
		return Config{
			TokensPerMinute: defaultTokensPerMinute,
			CostPerDay:      defaultCostPerDay,
		}
	}

	key := keyPrefixConfig + agentID
	vals, err := r.client.HGetAll(r.ctx, key).Result()
	if err != nil || len(vals) == 0 {
		return Config{
			TokensPerMinute: defaultTokensPerMinute,
			CostPerDay:      defaultCostPerDay,
		}
	}

	cfg := Config{
		TokensPerMinute: defaultTokensPerMinute,
		CostPerDay:      defaultCostPerDay,
	}

	if tpm, err := strconv.Atoi(vals["tokens_per_minute"]); err == nil {
		cfg.TokensPerMinute = tpm
	}
	if cpd, err := strconv.Atoi(vals["cost_per_day"]); err == nil {
		cfg.CostPerDay = cpd
	}

	return cfg
}

// SetAgentConfig updates the rate limiting configuration for a specific agent.
func (r *RedisRateLimiter) SetAgentConfig(agentID string, cfg Config) error {
	if r.client == nil {
		return fmt.Errorf("Redis not configured")
	}

	key := keyPrefixConfig + agentID
	return r.client.HSet(r.ctx, key, map[string]interface{}{
		"tokens_per_minute": cfg.TokensPerMinute,
		"cost_per_day":      cfg.CostPerDay,
	}).Err()
}

// GetAgentUsage returns the current token and cost usage for an agent in the current windows.
func (r *RedisRateLimiter) GetAgentUsage(agentID string) (tokensUsed int, costUsed int, err error) {
	if r.client == nil {
		return 0, 0, nil
	}

	now := time.Now().Unix()

	// Get minute token usage
	tokensKey := keyPrefixTokens + agentID
	r.client.ZRemRangeByScore(r.ctx, tokensKey, "0", strconv.FormatInt(now-int64(minuteWindow), 10))
	tokenEntries, err := r.client.ZRangeWithScores(r.ctx, tokensKey, 0, -1).Result()
	if err != nil {
		return 0, 0, err
	}
	for _, entry := range tokenEntries {
		tokensUsed += int(entry.Score) // Using score as a fallback; actual token count is in member
	}

	// Get daily cost usage
	budgetKey := keyPrefixBudget + agentID
	r.client.ZRemRangeByScore(r.ctx, budgetKey, "0", strconv.FormatInt(now-int64(dayWindow), 10))
	costEntries, err := r.client.ZRangeWithScores(r.ctx, budgetKey, 0, -1).Result()
	if err != nil {
		return 0, 0, err
	}
	for _, entry := range costEntries {
		costUsed += int(entry.Score)
	}

	return tokensUsed, costUsed, nil
}

// Close closes the Redis connection.
func (r *RedisRateLimiter) Close() error {
	if r.client != nil {
		return r.client.Close()
	}
	return nil
}
