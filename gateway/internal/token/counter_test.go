package token

import (
	"encoding/json"
	"testing"
)

func TestCountChatTokens_EmptyMessages(t *testing.T) {
	tokens := CountChatTokens("gpt-4", nil)
	if tokens != 3 {
		t.Errorf("expected 3 base tokens, got %d", tokens)
	}
}

func TestCountChatTokens_SimpleMessage(t *testing.T) {
	msg, _ := json.Marshal(map[string]string{
		"role":    "user",
		"content": "Hello, world!",
	})
	messages := []json.RawMessage{msg}

	tokens := CountChatTokens("gpt-4", messages)
	if tokens <= 0 {
		t.Errorf("expected positive token count, got %d", tokens)
	}
	// "Hello, world!" is ~3 tokens + role + overhead
	t.Logf("Token count for simple message: %d", tokens)
}

func TestCountChatTokens_MultipleMessages(t *testing.T) {
	systemMsg, _ := json.Marshal(map[string]string{
		"role":    "system",
		"content": "You are a helpful assistant.",
	})
	userMsg, _ := json.Marshal(map[string]string{
		"role":    "user",
		"content": "Tell me about AI governance in three sentences.",
	})

	messages := []json.RawMessage{systemMsg, userMsg}
	tokens := CountChatTokens("gpt-4", messages)
	if tokens <= 0 {
		t.Errorf("expected positive token count, got %d", tokens)
	}
	t.Logf("Token count for multiple messages: %d", tokens)

	// Same messages should count similarly for gpt-3.5
	tokens35 := CountChatTokens("gpt-3.5-turbo", messages)
	if tokens35 <= 0 {
		t.Errorf("expected positive token count for gpt-3.5, got %d", tokens35)
	}
	t.Logf("Token count for gpt-3.5: %d", tokens35)
}

func TestCountChatTokens_NoModel(t *testing.T) {
	msg, _ := json.Marshal(map[string]string{
		"role":    "user",
		"content": "Hello!",
	})
	messages := []json.RawMessage{msg}

	// Empty model should default to gpt-4 behavior
	tokens := CountChatTokens("", messages)
	if tokens <= 0 {
		t.Errorf("expected positive token count with empty model, got %d", tokens)
	}
}

func TestCountChatTokens_AnthropicModel(t *testing.T) {
	msg, _ := json.Marshal(map[string]string{
		"role":    "user",
		"content": "Hello, Claude!",
	})
	messages := []json.RawMessage{msg}

	// Anthropic models should use character-based estimation
	tokens := CountChatTokens("claude-3-opus-20240229", messages)
	if tokens <= 0 {
		t.Errorf("expected positive token count for claude, got %d", tokens)
	}
	t.Logf("Token count for claude model: %d", tokens)
}

func TestCountCompletionTokens(t *testing.T) {
	content := "This is a test completion response."
	tokens := CountCompletionTokens(content, "gpt-4")
	if tokens <= 0 {
		t.Errorf("expected positive token count, got %d", tokens)
	}
	t.Logf("Completion token count: %d", tokens)
}

func TestIsOpenAIModel(t *testing.T) {
	tests := []struct {
		model    string
		expected bool
	}{
		{"gpt-4", true},
		{"gpt-4-32k", true},
		{"gpt-3.5-turbo", true},
		{"gpt-3.5-turbo-0125", true},
		{"claude-3-opus", false},
		{"claude-2", false},
		{"", false},
		{"llama-2-70b", false},
	}

	for _, tt := range tests {
		result := isOpenAIModel(tt.model)
		if result != tt.expected {
			t.Errorf("isOpenAIModel(%q) = %v, want %v", tt.model, result, tt.expected)
		}
	}
}

func TestFormatModelName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"gpt-4", "gpt-4"},
		{"gpt-4-0125-preview", "gpt-4-turbo-preview"},
		{"gpt-3.5-turbo-0125", "gpt-3.5-turbo"},
		{"gpt-3.5-turbo-16k", "gpt-3.5-turbo"},
		{"claude-3-opus-20240229", "claude-3-opus-20240229"},
	}

	for _, tt := range tests {
		result := FormatModelName(tt.input)
		if result != tt.expected {
			t.Errorf("FormatModelName(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestEstimateCost(t *testing.T) {
	cost := EstimateCost("gpt-4", 100, 50)
	// 100/1000 * 0.03 + 50/1000 * 0.06 = 0.003 + 0.003 = 0.006
	expected := 0.006
	if cost != expected {
		t.Errorf("EstimateCost(gpt-4, 100, 50) = %f, want %f", cost, expected)
	}
}

func TestEstimateCost_GPT35(t *testing.T) {
	cost := EstimateCost("gpt-3.5-turbo", 1000, 200)
	// 1000/1000 * 0.0015 + 200/1000 * 0.002 = 0.0015 + 0.0004 = 0.0019
	expected := 0.0019
	if cost != expected {
		t.Errorf("EstimateCost(gpt-3.5, 1000, 200) = %f, want %f", cost, expected)
	}
}

func TestValidateMessages_Valid(t *testing.T) {
	msg, _ := json.Marshal(map[string]string{
		"role":    "user",
		"content": "Hello!",
	})
	err := ValidateMessages([]json.RawMessage{msg})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateMessages_MissingRole(t *testing.T) {
	msg, _ := json.Marshal(map[string]string{
		"content": "Hello!",
	})
	err := ValidateMessages([]json.RawMessage{msg})
	if err == nil {
		t.Error("expected error for missing role, got nil")
	}
}

func TestValidateMessages_Empty(t *testing.T) {
	err := ValidateMessages(nil)
	if err == nil {
		t.Error("expected error for empty messages, got nil")
	}
}

func TestCountContentTokens_MultiModal(t *testing.T) {
	// Simulate multi-modal content with text and image
	type contentPart struct {
		Type     string `json:"type"`
		Text     string `json:"text,omitempty"`
		ImageURL *struct {
			URL    string `json:"url"`
			Detail string `json:"detail,omitempty"`
		} `json:"image_url,omitempty"`
	}

	parts := []contentPart{
		{Type: "text", Text: "What's in this image?"},
		{Type: "image_url", ImageURL: &struct {
			URL    string `json:"url"`
			Detail string `json:"detail,omitempty"`
		}{
			URL:    "https://example.com/image.jpg",
			Detail: "low",
		}},
	}

	content, _ := json.Marshal(parts)

	msg := map[string]interface{}{
		"role":    "user",
		"content": content,
	}
	msgRaw, _ := json.Marshal(msg)

	tokens := CountChatTokens("gpt-4-vision-preview", []json.RawMessage{msgRaw})
	if tokens <= 0 {
		t.Errorf("expected positive token count for multi-modal, got %d", tokens)
	}
	t.Logf("Multi-modal token count: %d", tokens)
}
