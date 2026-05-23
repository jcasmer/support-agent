package core

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const resolverSystem = `You are a technical support specialist.
Your job is to resolve the user's support query based on its category.

Guidelines:
- Be concise and direct
- If you can resolve the issue, provide a clear step-by-step solution
- If you cannot resolve the issue with the information provided, respond with exactly: ESCALATE
- Never make up information you are not sure about
- Always acknowledge the category of the issue in your response`

const maxResolverTokens = 500

// ResolverAgent attempts to resolve the support query
type ResolverAgent struct {
	llm LLMPort
}

// NewResolverAgent creates a new ResolverAgent
func NewResolverAgent(llm LLMPort) *ResolverAgent {
	return &ResolverAgent{llm: llm}
}

// Type returns the agent type identifier
func (a *ResolverAgent) Type() AgentType {
	return AgentResolver
}

// Execute attempts to resolve the query and returns an updated state
func (a *ResolverAgent) Execute(ctx context.Context, state State) (AgentResult, error) {
	messages := buildMessages(state)

	req := CompletionRequest{
		System:    resolverSystem,
		Messages:  messages,
		MaxTokens: maxResolverTokens,
	}

	resp, err := a.llm.Complete(ctx, req)
	if err != nil {
		return AgentResult{}, fmt.Errorf("resolver llm call: %w", err)
	}

	newState := state
	newState.TokensUsed += resp.TokensUsed

	if shouldEscalate(resp.Content) {
		newState.Resolved = false
		newState.History = append(newState.History, Message{
			Role:    "assistant",
			Content: "resolver could not resolve the issue, escalating",
			At:      time.Now(),
		})
	} else {
		newState.Resolved = true
		newState.Response = resp.Content
		newState.History = append(newState.History, Message{
			Role:    "assistant",
			Content: resp.Content,
			At:      time.Now(),
		})
	}

	return AgentResult{
		NextState:  newState,
		AgentType:  AgentResolver,
		TokensUsed: resp.TokensUsed,
	}, nil
}

// buildMessages constructs the message history for the LLM
func buildMessages(state State) []Message {
	messages := []Message{
		{
			Role:    "user",
			Content: fmt.Sprintf("[category: %s] %s", state.Category, state.Query),
			At:      time.Now(),
		},
	}

	for _, m := range state.History {
		if strings.HasPrefix(m.Content, "classified as:") {
			continue
		}
		messages = append(messages, m)
	}

	return messages
}

// shouldEscalate checks if the LLM signaled it cannot resolve the issue
func shouldEscalate(response string) bool {
	return strings.Contains(strings.ToUpper(strings.TrimSpace(response)), "ESCALATE")
}