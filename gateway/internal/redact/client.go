package redact

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// SLMRedactorClient implements the proxy.Redactor interface by calling
// the on-premise SLM PII redaction service.
type SLMRedactorClient struct {
	baseURL    string
	httpClient *http.Client
	timeout    time.Duration
}

// RedactRequest is the payload sent to the SLM redactor service.
type RedactRequest struct {
	AgentID  string            `json:"agent_id,omitempty"`
	Messages []json.RawMessage `json:"messages"`
}

// RedactResponse is the response from the SLM redactor service.
type RedactResponse struct {
	AgentID          string            `json:"agent_id,omitempty"`
	Messages         []json.RawMessage `json:"messages"`
	RedactionsMade   int               `json:"redactions_made"`
	ProcessingTimeMs float64           `json:"processing_time_ms"`
}

// NewSLMRedactorClient creates a new client for the SLM PII redaction service.
// If baseURL is empty, it returns a no-op client that passes messages through.
func NewSLMRedactorClient(baseURL string) *SLMRedactorClient {
	if baseURL == "" {
		log.Println("SLM redactor URL not configured — PII redaction disabled")
		return &SLMRedactorClient{baseURL: ""}
	}

	return &SLMRedactorClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:       50,
				IdleConnTimeout:    30 * time.Second,
				DisableCompression: false,
			},
		},
		timeout: 5 * time.Second,
	}
}

// Redact sends messages to the SLM redactor service for PII detection and masking.
func (c *SLMRedactorClient) Redact(agentID string, messages []json.RawMessage) ([]json.RawMessage, error) {
	if c.baseURL == "" {
		// No redactor configured — pass through
		return messages, nil
	}

	req := RedactRequest{
		AgentID:  agentID,
		Messages: messages,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal redact request: %w", err)
	}

	httpReq, err := http.NewRequest(
		http.MethodPost,
		fmt.Sprintf("%s/redact", c.baseURL),
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create redact request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("redact request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("redact service returned %d: %s",
			resp.StatusCode, string(respBody))
	}

	var redactResp RedactResponse
	if err := json.NewDecoder(resp.Body).Decode(&redactResp); err != nil {
		return nil, fmt.Errorf("failed to decode redact response: %w", err)
	}

	if redactResp.RedactionsMade > 0 {
		log.Printf("[REDACT] Agent=%s: %d PII instances redacted (%.2fms)",
			agentID, redactResp.RedactionsMade, redactResp.ProcessingTimeMs)
	}

	return redactResp.Messages, nil
}

// RedactResponseContent sends LLM response text to the redactor for defense-in-depth.
func (c *SLMRedactorClient) RedactResponseContent(agentID string, content string) (string, error) {
	if c.baseURL == "" {
		return content, nil
	}

	req := map[string]interface{}{
		"agent_id": agentID,
		"content":  content,
	}

	body, _ := json.Marshal(req)
	httpReq, err := http.NewRequest(
		http.MethodPost,
		fmt.Sprintf("%s/redact/response", c.baseURL),
		bytes.NewReader(body),
	)
	if err != nil {
		return content, fmt.Errorf("failed to create response redact request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return content, fmt.Errorf("response redact request failed: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return content, err
	}

	return result.Content, nil
}

// HealthCheck verifies the SLM redactor service is reachable.
func (c *SLMRedactorClient) HealthCheck() error {
	if c.baseURL == "" {
		return fmt.Errorf("SLM redactor not configured")
	}

	resp, err := c.httpClient.Get(fmt.Sprintf("%s/health", c.baseURL))
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned %d", resp.StatusCode)
	}

	return nil
}
