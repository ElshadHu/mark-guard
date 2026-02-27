// Package llm handles communication with LLM APIs
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// ChatRequest is the request body for /v1/chat/completions
type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature float64       `json:"temperature"`
}

// ChatMessage is a single message in the conversation
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatResponse is the response from /v1/chat/completions
type ChatResponse struct {
	Choices []ChatChoice `json:"choices"`
	Usage   *Usage       `json:"usage,omitempty"`
	Error   *APIError    `json:"error,omitempty"`
}

// ChatChoice is a single completion choice
type ChatChoice struct {
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

// Usage reports token consumption for the request
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// APIError is an error returned in the response body
type APIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

// Client sends requests to an OpenAI-compatible LLM API
type Client struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewClient creates an LLM client from config values
// It resolves the API key from the env variables specified in apiKeyEnv
func NewClient(baseURL, apiKeyEnv, model string) (*Client, error) {
	apiKey, err := resolveAPIKey(apiKeyEnv)
	if err != nil {
		return nil, err
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		model:   model,
		httpClient: &http.Client{
			Timeout: 120 * time.Second, // LLM responses can be slow
		},
	}, nil
}

// resolveAPIKey reads the API key from the named env variable
func resolveAPIKey(envVar string) (string, error) {
	key := os.Getenv(envVar)
	if key == "" {
		return "", fmt.Errorf(
			"API key not found: set the %s environment variable\n"+
				"  Default (free): get a Gemini key at https://ai.google.dev/gemini-api/docs/api-key\n"+
				"  OpenAI: set OPENAI_API_KEY from https://platform.openai.com/api-keys",
			envVar,
		)
	}
	return key, nil
}

// Complete sends a chat completion request and returns the response
// retries on transient errors with exponential backoff
func (c *Client) Complete(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	req.Model = c.model

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	var lastErr error
	maxRetries := 3

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			delay := backoffDuration(attempt)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		resp, err := c.doRequest(ctx, body)
		if err != nil {
			lastErr = err
			// Network errors are retryable
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("request failed after %d retries: %w", maxRetries, lastErr)
}

// doRequest performs a single HTTP request to the LLM API
func (c *Client) doRequest(ctx context.Context, body []byte) (*ChatResponse, error) {
	url := c.baseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer func() { _ = httpResp.Body.Close() }()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	// Handle non-200 status codes
	if httpResp.StatusCode != http.StatusOK {
		return nil, c.handleErrorStatus(httpResp.StatusCode, httpResp.Header, respBody)
	}
	var chatResp ChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}
	// Check for error in response body
	if chatResp.Error != nil {
		return nil, fmt.Errorf("API error: %s (type: %s, code: %s)",
			chatResp.Error.Message, chatResp.Error.Type, chatResp.Error.Code)
	}
	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("API returned no choices")
	}
	return &chatResp, nil
}

// Error represents a categorized error from the LLM API.
type Error struct {
	StatusCode int
	Message    string
	Retryable  bool
	RetryAfter time.Duration
}

func (e *Error) Error() string {
	return fmt.Sprintf("LLM API error (HTTP %d): %s", e.StatusCode, e.Message)
}

// handleErrorStatus categorizes HTTP error responses
func (c *Client) handleErrorStatus(status int, headers http.Header, body []byte) *Error {
	// Try to parse error message from body
	msg := string(body)
	var apiResp ChatResponse
	if err := json.Unmarshal(body, &apiResp); err == nil && apiResp.Error != nil {
		msg = apiResp.Error.Message
	}
	llmErr := &Error{
		StatusCode: status,
		Message:    msg,
	}
	switch {
	case status == http.StatusUnauthorized:
		llmErr.Message = fmt.Sprintf("authentication failed: %s\nCheck that %s is set correctly",
			msg, "your API key environment variable")
		llmErr.Retryable = false
	case status == http.StatusTooManyRequests: // 429
		llmErr.Message = fmt.Sprintf("rate limited: %s", msg)
		llmErr.Retryable = true
		// Parse Retry-After header if present
		if ra := headers.Get("Retry-After"); ra != "" {
			if secs, err := strconv.Atoi(ra); err == nil {
				llmErr.RetryAfter = time.Duration(secs) * time.Second
			}
		}
	case status == http.StatusBadRequest:
		// Context length exceeded is a 400 with specific message
		if strings.Contains(msg, "context_length") || strings.Contains(msg, "maximum context") ||
			strings.Contains(msg, "too long") || strings.Contains(msg, "token") {
			llmErr.Message = fmt.Sprintf("context length exceeded: %s\n"+
				"Try narrowing scope with --path flag or add docs.mappings to .markguard.yaml", msg)
		}
		llmErr.Retryable = false
	case status >= 500:
		llmErr.Message = fmt.Sprintf("server error: %s", msg)
		llmErr.Retryable = true
	default:
		llmErr.Retryable = false
	}

	return llmErr
}

// backoffDuration returns the delay before retry attempt n (0-indexed)
// Uses exponential backoff with jitter: base * 2^attempt +- 25%.
func backoffDuration(attempt int) time.Duration {
	base := time.Duration(1<<uint(attempt)) * time.Second
	if base > 30*time.Second {
		base = 30 * time.Second
	}
	// Add jitter
	jitter := time.Duration(rand.Int63n(int64(base) / 4))
	return base + jitter
}
