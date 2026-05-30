package token

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pandodao/tokenizer-go"
)

// Message represents a single message in a chat completion request.
type Message struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"` // string or []ContentPart
}

// ContentPart represents a part of a multi-modal message.
type ContentPart struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	ImageURL *struct {
		URL    string `json:"url"`
		Detail string `json:"detail,omitempty"`
	} `json:"image_url,omitempty"`
}

// CountChatTokens estimates the number of tokens in a chat completion request.
// Uses OpenAI's tiktoken approximation via the pandodao/tokenizer-go library.
// For non-OpenAI models, falls back to character-based estimation.
func CountChatTokens(model string, messages []json.RawMessage) int {
	model = strings.TrimSpace(model)
	if model == "" {
		model = "gpt-4"
	}

	// Determine if we can use tiktoken for this model
	useTiktoken := isOpenAIModel(model)

	totalTokens := 0

	// Per-message overhead (approximate from OpenAI docs)
	// Every message follows: <|start|>{role}\n{content}<|end|>\n
	perMessageTokens := 3

	// Add assistant reply overhead if we're estimating total (completion)
	if model == "gpt-4" || model == "gpt-4-32k" {
		perMessageTokens = 3 // GPT-4 uses 3 tokens per message
	}

	for _, msgRaw := range messages {
		totalTokens += perMessageTokens

		var msg Message
		if err := json.Unmarshal(msgRaw, &msg); err != nil {
			// If we can't parse, just estimate from raw bytes
			totalTokens += estimateTokensFromBytes(msgRaw)
			continue
		}

		// Count tokens for the role name
		totalTokens += countTokensForString(msg.Role, useTiktoken)

		// Count tokens for content
		contentTokens := countContentTokens(msg.Content, model, useTiktoken)
		totalTokens += contentTokens
	}

	// Add base request overhead (approximate)
	// Every request has: <|im_start|>system\n{system_prompt}<|im_end|>
	totalTokens += 3 // base request formatting

	return totalTokens
}

// countContentTokens counts tokens for message content which can be
// a plain string or an array of content parts (multi-modal).
func countContentTokens(content json.RawMessage, model string, useTiktoken bool) int {
	// Try as string first
	var text string
	if err := json.Unmarshal(content, &text); err == nil {
		return countTokensForString(text, useTiktoken)
	}

	// Try as array of content parts
	var parts []ContentPart
	if err := json.Unmarshal(content, &parts); err != nil {
		// Fallback: estimate from raw bytes
		return estimateTokensFromBytes(content)
	}

	total := 0
	for _, part := range parts {
		switch part.Type {
		case "text":
			total += countTokensForString(part.Text, useTiktoken)
		case "image_url":
			// Image tokens: low detail = 85 tokens, high detail = 170 tokens
			// (simplified; actual is more complex based on image dimensions)
			if part.ImageURL != nil {
				detail := part.ImageURL.Detail
				switch detail {
				case "low":
					total += 85
				case "high", "auto":
					total += 170
				default:
					total += 85
				}
			} else {
				total += 85
			}
		default:
			// Unknown content type, estimate from JSON
			data, _ := json.Marshal(part)
			total += estimateTokensFromBytes(data)
		}
	}
	return total
}

// countTokensForString returns the token count for a string.
// Uses tiktoken for OpenAI models, character-based estimate for others.
func countTokensForString(text string, useTiktoken bool) int {
	if !useTiktoken {
		return estimateTokensFromString(text)
	}

	count, err := tokenizer.CalToken(text)
	if err != nil {
		// Fallback on error
		return estimateTokensFromString(text)
	}
	return count
}

// CountCompletionTokens estimates the number of tokens in a completion response.
// For chat models, this is the number of tokens in the assistant's reply.
func CountCompletionTokens(content string, model string) int {
	if isOpenAIModel(model) {
		count, err := tokenizer.CalToken(content)
		if err == nil {
			return count
		}
	}
	return estimateTokensFromString(content)
}

// isOpenAIModel returns true if the model is an OpenAI model that
// can be tokenized with tiktoken.
func isOpenAIModel(model string) bool {
	openaiModels := []string{
		"gpt-4", "gpt-4-32k", "gpt-4-turbo", "gpt-4-turbo-preview",
		"gpt-4-0125-preview", "gpt-4-1106-preview", "gpt-4-vision-preview",
		"gpt-3.5-turbo", "gpt-3.5-turbo-0125", "gpt-3.5-turbo-1106",
		"gpt-3.5-turbo-16k", "gpt-3.5-turbo-instruct",
		"text-davinci-003", "text-davinci-002",
		"text-embedding-3-small", "text-embedding-3-large",
		"text-embedding-ada-002",
	}

	for _, m := range openaiModels {
		if strings.HasPrefix(model, m) {
			return true
		}
	}
	return false
}

// estimateTokensFromString provides a rough token estimate (1 token ≈ 4 chars).
func estimateTokensFromString(s string) int {
	if len(s) == 0 {
		return 0
	}
	tokens := len(s) / 4
	if tokens < 1 {
		tokens = 1
	}
	return tokens
}

// estimateTokensFromBytes provides a rough token estimate from raw JSON bytes.
func estimateTokensFromBytes(data []byte) int {
	return len(data) / 4
}

// FormatModelName normalizes the model name for cost calculation.
func FormatModelName(rawModel string) string {
	// Strip version suffixes for cost matching
	model := strings.ToLower(rawModel)
	model = strings.TrimSpace(model)

	// Map common model aliases
	aliases := map[string]string{
		"gpt-4-turbo":            "gpt-4-turbo",
		"gpt-4":                  "gpt-4",
		"gpt-4-32k":              "gpt-4-32k",
		"gpt-4-0125-preview":     "gpt-4-turbo-preview",
		"gpt-4-1106-preview":     "gpt-4-turbo-preview",
		"gpt-4-vision-preview":   "gpt-4-vision-preview",
		"gpt-3.5-turbo":          "gpt-3.5-turbo",
		"gpt-3.5-turbo-0125":     "gpt-3.5-turbo",
		"gpt-3.5-turbo-1106":     "gpt-3.5-turbo",
		"gpt-3.5-turbo-16k":      "gpt-3.5-turbo",
		"gpt-3.5-turbo-instruct": "gpt-3.5-turbo-instruct",
	}

	if alias, ok := aliases[model]; ok {
		return alias
	}
	return model
}

// EstimateCost calculates the dollar cost of an LLM API call.
func EstimateCost(model string, promptTokens, completionTokens int) float64 {
	model = FormatModelName(model)

	// Pricing per 1K tokens (as of early 2024)
	pricing := map[string][2]float64{
		"gpt-4":                  {0.03, 0.06}, // prompt, completion
		"gpt-4-32k":              {0.06, 0.12},
		"gpt-4-turbo-preview":    {0.01, 0.03},
		"gpt-4-vision-preview":   {0.01, 0.03},
		"gpt-3.5-turbo":          {0.0015, 0.002},
		"gpt-3.5-turbo-instruct": {0.0015, 0.002},
	}

	rates, ok := pricing[model]
	if !ok {
		// Default: use GPT-4 pricing
		rates = [2]float64{0.03, 0.06}
	}

	return (float64(promptTokens)/1000)*rates[0] + (float64(completionTokens)/1000)*rates[1]
}

// StringPtr is a helper to create a *string from a string literal.
func StringPtr(s string) *string {
	return &s
}

// ValidateMessages validates the structure of chat messages and returns
// a descriptive error if something is wrong.
func ValidateMessages(messages []json.RawMessage) error {
	if len(messages) == 0 {
		return fmt.Errorf("messages array is empty")
	}
	for i, msg := range messages {
		var m map[string]interface{}
		if err := json.Unmarshal(msg, &m); err != nil {
			return fmt.Errorf("message %d: invalid JSON: %w", i, err)
		}
		if _, ok := m["role"]; !ok {
			return fmt.Errorf("message %d: missing 'role' field", i)
		}
		if _, ok := m["content"]; !ok {
			return fmt.Errorf("message %d: missing 'content' field", i)
		}
	}
	return nil
}
