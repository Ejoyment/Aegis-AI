package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

const (
	// AgentIDHeader is the header carrying the AI agent's cryptographic identity.
	AgentIDHeader = "X-Aegis-Agent-ID"
)

// RequestLog represents a proxied request's metadata for audit logging.
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

// Proxy holds the dependencies for the reverse proxy.
type Proxy struct {
	config    *Config
	upstream  *url.URL
	proxy     *httputil.ReverseProxy
	rateLimit RateLimiter
	audit     AuditLogger
	redact    Redactor
}

// RateLimiter defines the interface for token-based rate limiting.
type RateLimiter interface {
	Allow(agentID string, tokens int) (bool, int, error) // allowed, remaining, error
}

// AuditLogger defines the interface for persisting request audit logs.
type AuditLogger interface {
	Log(entry *RequestLog) error
}

// Redactor defines the interface for PII redaction.
type Redactor interface {
	Redact(agentID string, messages []json.RawMessage) ([]json.RawMessage, error)
}

// OpenAIRequest is the minimal structure we need to inspect from LLM requests.
type OpenAIRequest struct {
	Model     string            `json:"model"`
	Messages  []json.RawMessage `json:"messages"`
	MaxTokens int               `json:"max_tokens,omitempty"`
	Stream    bool              `json:"stream,omitempty"`
}

// OpenAIResponse is the minimal structure for extracting token usage.
type OpenAIResponse struct {
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage,omitempty"`
	Model string `json:"model"`
}

// NewProxy creates a new Proxy instance.
func NewProxy(cfg *Config, rl RateLimiter, al AuditLogger, red Redactor) (*Proxy, error) {
	upstream, err := url.Parse(cfg.UpstreamURL)
	if err != nil {
		return nil, fmt.Errorf("invalid upstream URL: %w", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(upstream)

	// Modify the upstream request to include the API key
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = upstream.Host
		if cfg.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
		}
		// Mark this as a proxied request
		req.Header.Set("X-Aegis-Proxy", "true")
	}

	return &Proxy{
		config:    cfg,
		upstream:  upstream,
		proxy:     proxy,
		rateLimit: rl,
		audit:     al,
		redact:    red,
	}, nil
}

// ServeHTTP implements the http.Handler interface. It is the main entry point
// for all proxied LLM requests.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	requestID := fmt.Sprintf("aegis-%d", time.Now().UnixNano())

	// Extract agent identity
	agentID := r.Header.Get(AgentIDHeader)
	if agentID == "" {
		agentID = "unknown"
	}

	log.Printf("[%s] Request: %s %s | Agent: %s", requestID, r.Method, r.URL.Path, agentID)

	// Only intercept chat/completions and completions endpoints
	if !isLLMEndpoint(r.URL.Path) {
		p.proxy.ServeHTTP(w, r)
		return
	}

	// Read and parse the request body
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	r.Body.Close()
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	var openAIReq OpenAIRequest
	if err := json.Unmarshal(bodyBytes, &openAIReq); err != nil {
		log.Printf("[%s] Failed to parse request body: %v", requestID, err)
		p.proxy.ServeHTTP(w, r)
		return
	}

	// TODO: Week 2 — Estimate token count from messages
	promptTokens := estimateTokens(openAIReq.Messages, openAIReq.Model)

	// TODO: Week 3 — Check rate limit
	if p.rateLimit != nil {
		allowed, remaining, err := p.rateLimit.Allow(agentID, promptTokens)
		if err != nil {
			log.Printf("[%s] Rate limit check error: %v", requestID, err)
		} else if !allowed {
			log.Printf("[%s] Rate limit exceeded for agent %s", requestID, agentID)
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "rate_limit_exceeded",
				"message": fmt.Sprintf("Agent '%s' has exceeded its token budget. "+
					"Reduce request frequency or increase the limit.", agentID),
				"agent_id":            agentID,
				"retry_after_seconds": 60,
			})
			return
		}
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
	}

	// TODO: Week 6 — Redact PII from messages
	if p.redact != nil {
		redactedMessages, err := p.redact.Redact(agentID, openAIReq.Messages)
		if err != nil {
			log.Printf("[%s] Redaction error (proceeding without): %v", requestID, err)
		} else {
			// Rebuild the request body with redacted messages
			openAIReq.Messages = redactedMessages
			newBody, _ := json.Marshal(openAIReq)
			r.Body = io.NopCloser(bytes.NewBuffer(newBody))
			r.ContentLength = int64(len(newBody))
		}
	} else {
		// Restore original body for upstream
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	}

	// Capture the upstream response for audit logging
	recorder := &responseRecorder{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		body:           new(bytes.Buffer),
	}

	p.proxy.ServeHTTP(recorder, r)

	// Parse the response to extract token usage
	var openAIResp OpenAIResponse
	completionTokens := 0
	if err := json.Unmarshal(recorder.body.Bytes(), &openAIResp); err == nil && openAIResp.Usage != nil {
		promptTokens = openAIResp.Usage.PromptTokens
		completionTokens = openAIResp.Usage.CompletionTokens
	}

	// Calculate cost (simplified: $0.01 per 1K prompt tokens, $0.03 per 1K completion tokens for GPT-4)
	cost := calculateCost(openAIReq.Model, promptTokens, completionTokens)

	// Log to audit trail
	if p.audit != nil {
		logEntry := &RequestLog{
			AgentID:          agentID,
			Model:            openAIReq.Model,
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalCost:        cost,
			RequestID:        requestID,
			StatusCode:       recorder.statusCode,
			Timestamp:        time.Now().UTC().Format(time.RFC3339),
		}
		if err := p.audit.Log(logEntry); err != nil {
			log.Printf("[%s] Audit log error: %v", requestID, err)
		}
	}

	elapsed := time.Since(startTime)
	log.Printf("[%s] Completed: %s | Agent: %s | Model: %s | Tokens: %d+%d | Cost: $%.6f | Duration: %s",
		requestID, r.URL.Path, agentID, openAIReq.Model, promptTokens, completionTokens, cost, elapsed)
}

// responseRecorder wraps http.ResponseWriter to capture the response status and body.
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

func (r *responseRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

// isLLMEndpoint returns true if the path matches an LLM completion endpoint.
func isLLMEndpoint(path string) bool {
	return path == "/v1/chat/completions" || path == "/v1/completions"
}

// estimateTokens is a placeholder that will be replaced with actual token counting in Week 2.
func estimateTokens(messages []json.RawMessage, model string) int {
	// Rough estimate: 4 characters ≈ 1 token
	total := 0
	for _, msg := range messages {
		total += len(msg) / 4
	}
	return total
}

// calculateCost computes the approximate cost of an LLM request.
func calculateCost(model string, promptTokens, completionTokens int) float64 {
	// Default GPT-4 pricing
	promptRate := 0.03     // $ per 1K tokens
	completionRate := 0.06 // $ per 1K tokens

	// GPT-3.5 Turbo is cheaper
	if model == "gpt-3.5-turbo" || model == "gpt-3.5-turbo-0125" {
		promptRate = 0.0015
		completionRate = 0.002
	}

	return (float64(promptTokens)/1000)*promptRate + (float64(completionTokens)/1000)*completionRate
}
