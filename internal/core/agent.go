package core

import "context"

// Agent is the interface all agents must implement.
// The orchestrator only depends on this interface, never on concrete implementations.
type Agent interface {
	Type() AgentType
	Execute(ctx context.Context, state State) (AgentResult, error)
}

// AgentResult is what each agent returns to the orchestrator
type AgentResult struct {
	NextState  State
	AgentType  AgentType
	TokensUsed int
}

// LLMPort is the provider-agnostic port for language model calls.
// Adapters (anthropic, gemini) implement this port.
type LLMPort interface {
	Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
}