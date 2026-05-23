package gemini

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
	apiURL = "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent"
)

// Client implements core.LLMPort using the Gemini API
type Client struct {
	apiKey     string
	httpClient *http.Client
}

// New creates a new Gemini client
func New(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// --- Gemini API request/response shapes ---

type apiPart struct {
	Text string `json:"text"`
}

type apiContent struct {
	Role  string    `json:"role"`
	Parts []apiPart `json:"parts"`
}

type systemInstruction struct {
	Parts []apiPart `json:"parts"`
}

type generationConfig struct {
	MaxOutputTokens int `json:"maxOutputTokens"`
}

type apiRequest struct {
	SystemInstruction systemInstruction `json:"system_instruction"`
	Contents          []apiContent      `json:"contents"`
	GenerationConfig  generationConfig  `json:"generationConfig"`
}

type apiCandidate struct {
	Content apiContent `json:"content"`
}

type usageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
}

type apiResponse struct {
	Candidates    []apiCandidate `json:"candidates"`
	UsageMetadata usageMetadata  `json:"usageMetadata"`
	Error         *apiError      `json:"error,omitempty"`
}

type apiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
}

// Complete sends a completion request to the Gemini API
func (c *Client) Complete(ctx context.Context, req core.CompletionRequest) (core.CompletionResponse, error) {
	contents := make([]apiContent, len(req.Messages))
	for i, m := range req.Messages {
		role := m.Role
		if role == "assistant" {
			role = "model"
		}
		contents[i] = apiContent{
			Role:  role,
			Parts: []apiPart{{Text: m.Content}},
		}
	}

	body := apiRequest{
		SystemInstruction: systemInstruction{
			Parts: []apiPart{{Text: req.System}},
		},
		Contents: contents,
		GenerationConfig: generationConfig{
			MaxOutputTokens: req.MaxTokens,
		},
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return core.CompletionResponse{}, fmt.Errorf("marshaling request: %w", err)
	}

	url := fmt.Sprintf("%s?key=%s", apiURL, c.apiKey)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return core.CompletionResponse{}, fmt.Errorf("creating request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

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
		return core.CompletionResponse{}, fmt.Errorf("gemini api error [%s]: %s", apiResp.Error.Status, apiResp.Error.Message)
	}

	if resp.StatusCode != http.StatusOK {
		return core.CompletionResponse{}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	if len(apiResp.Candidates) == 0 || len(apiResp.Candidates[0].Content.Parts) == 0 {
		return core.CompletionResponse{}, fmt.Errorf("empty response from api")
	}

	tokensUsed := apiResp.UsageMetadata.PromptTokenCount + apiResp.UsageMetadata.CandidatesTokenCount

	return core.CompletionResponse{
		Content:    apiResp.Candidates[0].Content.Parts[0].Text,
		TokensUsed: tokensUsed,
	}, nil
}