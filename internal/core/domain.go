package core

import "time"

// AgentType identifies which agent executed an action
type AgentType string

const (
	AgentClassifier AgentType = "classifier"
	AgentResolver   AgentType = "resolver"
	AgentEscalator  AgentType = "escalator"
)

// Category is the output of the Classifier Agent
type Category string

const (
	CategoryBilling   Category = "billing"
	CategoryTechnical Category = "technical"
	CategoryGeneral   Category = "general"
	CategoryUnknown   Category = "unknown"
)

// Message represents a single turn in the conversation
type Message struct {
	Role    string
	Content string
	At      time.Time
}

// State holds the complete conversational state.
// It is passed by value — no agent mutates the original state.
type State struct {
	SessionID  string
	Query      string
	Category   Category
	History    []Message
	Resolved   bool
	Response   string
	TicketID   string
	TokensUsed int
}

// Result is the final output of an orchestration run
type Result struct {
	SessionID  string
	Response   string
	Category   string
	Resolved   bool
	TicketID   string
	TokensUsed int
	Duration   time.Duration
}

// Ticket represents a structured support escalation ticket
type Ticket struct {
	TicketID      string `json:"ticket_id"`
	Priority      string `json:"priority"`
	Summary       string `json:"summary"`
	Details       string `json:"details"`
	SuggestedTeam string `json:"suggested_team"`
	NextSteps     string `json:"next_steps"`
}

// CompletionRequest is a provider-agnostic LLM request
type CompletionRequest struct {
	System    string
	Messages  []Message
	MaxTokens int
}

// CompletionResponse is a provider-agnostic LLM response
type CompletionResponse struct {
	Content    string
	TokensUsed int
}