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
	AgentIDHeader = "X-Aegis-Agent-ID"
)

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

type Proxy struct {
	config    *Config
	upstream  *url.URL
	proxy     *httputil.ReverseProxy
	rateLimit RateLimiter
	audit     AuditLogger
	redact    Redactor
}

type RateLimiter interface {
	Allow(agentID string, tokens int) (bool, int, error)
}

type AuditLogger interface {
	Log(entry *RequestLog) error
}

type Redactor interface {
	Redact(agentID string, messages []json.RawMessage) ([]json.RawMessage, error)
}

type OpenAIRequest struct {
	Model     string            `json:"model"`
	Messages  []json.RawMessage `json:"messages"`
	MaxTokens int               `json:"max_tokens,omitempty"`
	Stream    bool              `json:"stream,omitempty"`
}

type OpenAIResponse struct {
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage,omitempty"`
	Model string `json:"model"`
}

func NewProxy(cfg *Config, rl RateLimiter, al AuditLogger, red Redactor) (*Proxy, error) {
	upstream, err := url.Parse(cfg.UpstreamURL)
	if err != nil {
		return nil, fmt.Errorf("invalid upstream URL: %w", err)
	}
	proxy := httputil.NewSingleHostReverseProxy(upstream)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = upstream.Host
		if cfg.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
		}
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

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Health check endpoint
	if r.URL.Path == "/health" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "ok",
			"service": "aegis-gateway",
			"version": "1.0.0",
		})
		return
	}

	// Metrics endpoint (placeholder for Prometheus)
	if r.URL.Path == "/metrics" {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, `# AegisAI Gateway Metrics
# TYPE aegis_requests_total counter
aegis_requests_total 0
# TYPE aegis_rate_limits_total counter
aegis_rate_limits_total 0
# TYPE aegis_tokens_used_total counter
aegis_tokens_used_total 0
# TYPE aegis_upstream_latency_seconds histogram
aegis_upstream_latency_seconds_bucket{le="0.05"} 0
aegis_upstream_latency_seconds_bucket{le="0.1"} 0
aegis_upstream_latency_seconds_bucket{le="+Inf"} 0
`)
		return
	}

	// Stats API endpoints
	if r.URL.Path == "/api/v1/stats/totals" || r.URL.Path == "/api/v1/stats/agents" ||
		r.URL.Path == "/api/v1/stats/logs" {
		p.handleStatsAPI(w, r)
		return
	}

	// Regular proxy flow
	startTime := time.Now()
	requestID := fmt.Sprintf("aegis-%d", time.Now().UnixNano())
	agentID := r.Header.Get(AgentIDHeader)
	if agentID == "" {
		agentID = "unknown"
	}
	log.Printf("[%s] Request: %s %s | Agent: %s", requestID, r.Method, r.URL.Path, agentID)

	if !isLLMEndpoint(r.URL.Path) {
		p.proxy.ServeHTTP(w, r)
		return
	}

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

	promptTokens := estimateTokens(openAIReq.Messages, openAIReq.Model)

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
				"error":               "rate_limit_exceeded",
				"message":             fmt.Sprintf("Agent '%s' has exceeded its token budget.", agentID),
				"agent_id":            agentID,
				"retry_after_seconds": 60,
			})
			return
		}
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
	}

	if p.redact != nil {
		redactedMessages, err := p.redact.Redact(agentID, openAIReq.Messages)
		if err != nil {
			log.Printf("[%s] Redaction error (proceeding without): %v", requestID, err)
		} else {
			openAIReq.Messages = redactedMessages
			newBody, _ := json.Marshal(openAIReq)
			r.Body = io.NopCloser(bytes.NewBuffer(newBody))
			r.ContentLength = int64(len(newBody))
		}
	} else {
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	}

	recorder := &responseRecorder{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		body:           new(bytes.Buffer),
	}
	p.proxy.ServeHTTP(recorder, r)

	var openAIResp OpenAIResponse
	completionTokens := 0
	if err := json.Unmarshal(recorder.body.Bytes(), &openAIResp); err == nil && openAIResp.Usage != nil {
		promptTokens = openAIResp.Usage.PromptTokens
		completionTokens = openAIResp.Usage.CompletionTokens
	}

	cost := calculateCost(openAIReq.Model, promptTokens, completionTokens)

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

func (p *Proxy) handleStatsAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	switch r.URL.Path {
	case "/api/v1/stats/totals":
		json.NewEncoder(w).Encode(map[string]interface{}{
			"total_tokens":   0,
			"total_cost":     0,
			"total_requests": 0,
			"agent_count":    0,
		})
	case "/api/v1/stats/agents":
		json.NewEncoder(w).Encode([]interface{}{})
	case "/api/v1/stats/logs":
		json.NewEncoder(w).Encode([]interface{}{})
	}
}

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

func isLLMEndpoint(path string) bool {
	return path == "/v1/chat/completions" || path == "/v1/completions"
}

func estimateTokens(messages []json.RawMessage, model string) int {
	total := 0
	for _, msg := range messages {
		total += len(msg) / 4
	}
	return total
}

func calculateCost(model string, promptTokens, completionTokens int) float64 {
	promptRate := 0.03
	completionRate := 0.06
	if model == "gpt-3.5-turbo" || model == "gpt-3.5-turbo-0125" {
		promptRate = 0.0015
		completionRate = 0.002
	}
	return (float64(promptTokens)/1000)*promptRate + (float64(completionTokens)/1000)*completionRate
}
