package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const escalatorSystem = `You are a support ticket generator.
Your job is to create a structured support ticket from the user's query.

You must respond with a valid JSON object using exactly this shape:
{
  "ticket_id": "a short unique identifier, e.g. TKT-20240523-001",
  "priority": "low | medium | high | critical",
  "summary": "one sentence describing the issue",
  "details": "full description of the problem with all relevant context",
  "suggested_team": "billing | engineering | customer-success",
  "next_steps": "recommended actions for the support team"
}

Do not include any text outside the JSON object.`

const maxEscalatorTokens = 400

// EscalatorAgent generates a structured support ticket when the resolver fails
type EscalatorAgent struct {
	llm LLMPort
}

// NewEscalatorAgent creates a new EscalatorAgent
func NewEscalatorAgent(llm LLMPort) *EscalatorAgent {
	return &EscalatorAgent{llm: llm}
}

// Type returns the agent type identifier
func (a *EscalatorAgent) Type() AgentType {
	return AgentEscalator
}

// Execute generates a support ticket and returns an updated state
func (a *EscalatorAgent) Execute(ctx context.Context, state State) (AgentResult, error) {
	req := CompletionRequest{
		System: escalatorSystem,
		Messages: []Message{
			{
				Role:    "user",
				Content: buildEscalationPrompt(state),
				At:      time.Now(),
			},
		},
		MaxTokens: maxEscalatorTokens,
	}

	resp, err := a.llm.Complete(ctx, req)
	if err != nil {
		return AgentResult{}, fmt.Errorf("escalator llm call: %w", err)
	}

	ticket, err := parseTicket(resp.Content)
	if err != nil {
		return AgentResult{}, fmt.Errorf("parsing escalation ticket: %w", err)
	}

	newState := state
	newState.TicketID = ticket.TicketID
	newState.Response = buildEscalationResponse(ticket)
	newState.TokensUsed += resp.TokensUsed
	newState.History = append(newState.History, Message{
		Role:    "assistant",
		Content: fmt.Sprintf("ticket created: %s", ticket.TicketID),
		At:      time.Now(),
	})

	return AgentResult{
		NextState:  newState,
		AgentType:  AgentEscalator,
		TokensUsed: resp.TokensUsed,
	}, nil
}

// buildEscalationPrompt builds the full context for the escalator LLM call
func buildEscalationPrompt(state State) string {
	return fmt.Sprintf(
		"Category: %s\nOriginal query: %s\nResolution attempts: the resolver was unable to handle this issue.",
		state.Category,
		state.Query,
	)
}

// parseTicket extracts a Ticket from the raw LLM response
func parseTicket(raw string) (Ticket, error) {
	cleaned := strings.TrimSpace(raw)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)

	var ticket Ticket
	if err := json.Unmarshal([]byte(cleaned), &ticket); err != nil {
		return Ticket{}, fmt.Errorf("invalid ticket json: %w", err)
	}

	if ticket.TicketID == "" {
		return Ticket{}, fmt.Errorf("ticket missing required field: ticket_id")
	}

	return ticket, nil
}

// buildEscalationResponse formats the ticket into a human-readable response
func buildEscalationResponse(ticket Ticket) string {
	return fmt.Sprintf(
		"Your issue has been escalated to our %s team.\n\nTicket ID: %s\nPriority: %s\nSummary: %s\n\nNext steps: %s",
		ticket.SuggestedTeam,
		ticket.TicketID,
		ticket.Priority,
		ticket.Summary,
		ticket.NextSteps,
	)
}