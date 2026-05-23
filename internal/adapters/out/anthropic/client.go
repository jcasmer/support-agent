package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/jcasmer/support-agent/internal/core"
)

const (
	apiURL     = "https://api.anthropic.com/v1/messages"
	apiVersion = "2023-06-01"
	model      = "claude-haiku-4-5-20251001"
)

// Client implements core.LLMPort using the Anthropic Messages API
type Client struct {
	apiKey     string
	httpClient *http.Client
}

// New creates a new Anthropic client
func New(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// --- Anthropic API request/response shapes ---

type apiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type apiRequest struct {
	Model     string       `json:"model"`
	MaxTokens int          `json:"max_tokens"`
	System    string       `json:"system,omitempty"`
	Messages  []apiMessage `json:"messages"`
}

type apiContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type apiUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type apiResponse struct {
	Content []apiContent `json:"content"`
	Usage   apiUsage     `json:"usage"`
	Error   *apiError    `json:"error,omitempty"`
}

type apiError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// Complete sends a completion request to the Anthropic API
func (c *Client) Complete(ctx context.Context, req core.CompletionRequest) (core.CompletionResponse, error) {
	messages := make([]apiMessage, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = apiMessage{
			Role:    m.Role,
			Content: m.Content,
		}
	}

	body := apiRequest{
		Model:     model,
		MaxTokens: req.MaxTokens,
		System:    req.System,
		Messages:  messages,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return core.CompletionResponse{}, fmt.Errorf("marshaling request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(payload))
	if err != nil {
		return core.CompletionResponse{}, fmt.Errorf("creating request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", apiVersion)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return core.CompletionResponse{}, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	var apiResp apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return core.CompletionResponse{}, fmt.Errorf("decoding response: %w", err)
	}

	if apiResp.Error != nil {
		return core.CompletionResponse{}, fmt.Errorf("anthropic api error [%s]: %s", apiResp.Error.Type, apiResp.Error.Message)
	}

	if resp.StatusCode != http.StatusOK {
		return core.CompletionResponse{}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	if len(apiResp.Content) == 0 {
		return core.CompletionResponse{}, fmt.Errorf("empty response from api")
	}

	return core.CompletionResponse{
		Content:    apiResp.Content[0].Text,
		TokensUsed: apiResp.Usage.InputTokens + apiResp.Usage.OutputTokens,
	}, nil
}