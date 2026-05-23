package app

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jcasmer/support-agent/internal/core"
)

// Orchestrator coordinates the execution of agents in sequence
type Orchestrator struct {
	classifier core.Agent
	resolver   core.Agent
	escalator  core.Agent
}

// New creates a new Orchestrator with the provided agents
func New(
	classifier core.Agent,
	resolver core.Agent,
	escalator core.Agent,
) *Orchestrator {
	return &Orchestrator{
		classifier: classifier,
		resolver:   resolver,
		escalator:  escalator,
	}
}

// Run executes the full agent pipeline for a given query.
// Flow: Classifier → Resolver → Escalator (only if resolver fails)
func (o *Orchestrator) Run(ctx context.Context, sessionID, query string) (core.Result, error) {
	start := time.Now()

	state := core.State{
		SessionID: sessionID,
		Query:     query,
		History: []core.Message{
			{
				Role:    "user",
				Content: query,
				At:      time.Now(),
			},
		},
	}

	slog.Info("orchestration started",
		"session_id", sessionID,
		"query_length", len(query),
	)

	// Step 1: Classify
	state, err := o.runAgent(ctx, o.classifier, state)
	if err != nil {
		return core.Result{}, fmt.Errorf("classification step: %w", err)
	}

	slog.Info("classification complete",
		"session_id", sessionID,
		"category", state.Category,
	)

	// Step 2: Resolve
	state, err = o.runAgent(ctx, o.resolver, state)
	if err != nil {
		return core.Result{}, fmt.Errorf("resolution step: %w", err)
	}

	slog.Info("resolution complete",
		"session_id", sessionID,
		"resolved", state.Resolved,
	)

	// Step 3: Escalate only if resolver could not resolve
	if !state.Resolved {
		slog.Info("escalating issue",
			"session_id", sessionID,
			"category", state.Category,
		)

		state, err = o.runAgent(ctx, o.escalator, state)
		if err != nil {
			return core.Result{}, fmt.Errorf("escalation step: %w", err)
		}

		slog.Info("escalation complete",
			"session_id", sessionID,
			"ticket_id", state.TicketID,
		)
	}

	duration := time.Since(start)

	slog.Info("orchestration complete",
		"session_id", sessionID,
		"tokens_used", state.TokensUsed,
		"duration_ms", duration.Milliseconds(),
		"resolved", state.Resolved,
	)

	return core.Result{
		SessionID:  sessionID,
		Response:   state.Response,
		Category:   string(state.Category),
		TicketID:   state.TicketID,
		Resolved:   state.Resolved,
		TokensUsed: state.TokensUsed,
		Duration:   duration,
	}, nil
}

// runAgent executes a single agent and returns the updated state.
// It is the single place where state transitions are accepted.
func (o *Orchestrator) runAgent(ctx context.Context, a core.Agent, state core.State) (core.State, error) {
	slog.Debug("running agent",
		"agent", a.Type(),
		"session_id", state.SessionID,
	)

	result, err := a.Execute(ctx, state)
	if err != nil {
		return state, fmt.Errorf("agent %s: %w", a.Type(), err)
	}

	return result.NextState, nil
}