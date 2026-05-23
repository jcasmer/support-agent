package core

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const classifierSystem = `You are a support ticket classifier.
Your only job is to classify the user's query into exactly one of these categories:
- billing: payment issues, invoices, charges, refunds, subscriptions
- technical: bugs, errors, integrations, API issues, performance problems
- general: account settings, how-to questions, feature requests, everything else

Respond with a single word: billing, technical, or general.
Do not include any explanation or punctuation.`

// ClassifierAgent classifies the user query into a support category
type ClassifierAgent struct {
	llm LLMPort
}

// NewClassifierAgent creates a new ClassifierAgent
func NewClassifierAgent(llm LLMPort) *ClassifierAgent {
	return &ClassifierAgent{llm: llm}
}

// Type returns the agent type identifier
func (a *ClassifierAgent) Type() AgentType {
	return AgentClassifier
}

// Execute classifies the query and returns an updated state with the category set
func (a *ClassifierAgent) Execute(ctx context.Context, state State) (AgentResult, error) {
	req := CompletionRequest{
		System: classifierSystem,
		Messages: []Message{
			{
				Role:    "user",
				Content: state.Query,
				At:      time.Now(),
			},
		},
		MaxTokens: 10,
	}

	resp, err := a.llm.Complete(ctx, req)
	if err != nil {
		return AgentResult{}, fmt.Errorf("classifier llm call: %w", err)
	}

	category := parseCategory(resp.Content)

	newState := state
	newState.Category = category
	newState.History = append(newState.History, Message{
		Role:    "assistant",
		Content: fmt.Sprintf("classified as: %s", category),
		At:      time.Now(),
	})
	newState.TokensUsed += resp.TokensUsed

	return AgentResult{
		NextState:  newState,
		AgentType:  AgentClassifier,
		TokensUsed: resp.TokensUsed,
	}, nil
}

// parseCategory normalizes the LLM response into a known Category
func parseCategory(raw string) Category {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "billing":
		return CategoryBilling
	case "technical":
		return CategoryTechnical
	case "general":
		return CategoryGeneral
	default:
		return CategoryUnknown
	}
}